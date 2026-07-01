package flow

import (
	"context"
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/ai"
	"agent-flow/internal/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Conversation 对话结构，存储对话历史
type Conversation struct {
	ID        string
	Name      string    // 对话名称
	Rules     string    // 对话规则/系统指令
	Messages  []Message // 对话历史
	CreatedAt metav1.Time
	UpdatedAt metav1.Time
	Context   context.Context // 对话上下文
}

// Message 消息结构
type Message struct {
	Role      string      `json:"role"`
	Content   string      `json:"content"`
	Timestamp metav1.Time `json:"timestamp"`
}

// TaskRequest 任务请求，由架构生成，用户批准
type TaskRequest struct {
	ID                string           `json:"id"`
	Description       string           `json:"description"`
	Rationale         string           `json:"rationale,omitempty"`
	Priority          int              `json:"priority"`
	Resources         ResourceEstimate `json:"resources"`
	NeedsQualityCheck bool             `json:"needs_quality_check"`
	CreatedAt         metav1.Time      `json:"created_at"`
	Approved          bool             `json:"approved"`
	ApprovedAt        *metav1.Time     `json:"approved_at,omitempty"`
	ApprovedBy        string           `json:"approved_by,omitempty"`
	RejectionReason   string           `json:"rejection_reason,omitempty"`
}

// ResourceEstimate 资源预估
type ResourceEstimate struct {
	CPU      string `json:"cpu"`
	Memory   string `json:"memory"`
	Duration int    `json:"duration"`
}

// ChatRouter 架构对话路由接口，处理用户与架构的对话
type ChatRouter struct {
	mu            sync.RWMutex
	currentConv   *Conversation
	conversations map[string]*Conversation
	client        client.Client
	scheme        interface{}

	// 任务队列，待批准的任务
	pendingTasks []*TaskRequest
	taskQueueMu  sync.RWMutex

	// 工作流队列，待批准的工作流
	pendingWorkflows []*WorkflowRequest

	// 系统角色配置
	systemRole string // 系统角色/指令
	maxHistory int    // 最大对话历史

	// AI 服务
	aiService *ai.Service

	// 中间状态存储（Redis 或内存）
	store cache.StateStore
}

// NewChatRouter 创建新的对话路由
func NewChatRouter(client client.Client, scheme interface{}, aiSvc *ai.Service, store cache.StateStore) *ChatRouter {
	if store == nil {
		store = cache.NewMemoryStore()
	}
	r := &ChatRouter{
		conversations: make(map[string]*Conversation),
		client:        client,
		scheme:        scheme,
		aiService:     aiSvc,
		store:         store,
		systemRole:    "你是一个任务编排助手，负责帮助用户分析和分配任务。",
		maxHistory:    50,
	}
	r.loadStateFromStore(context.Background())
	return r
}

// StateStore returns the backing state store.
func (r *ChatRouter) StateStore() cache.StateStore {
	return r.store
}

// CreateConversation 创建新对话
func (r *ChatRouter) CreateConversation(name, rules string) *Conversation {
	r.mu.Lock()
	defer r.mu.Unlock()

	conv := &Conversation{
		ID:        fmt.Sprintf("conv-%d", len(r.conversations)+1),
		Name:      name,
		Rules:     rules,
		Messages:  []Message{},
		CreatedAt: metav1.Now(),
		UpdatedAt: metav1.Now(),
	}

	r.conversations[conv.ID] = conv
	r.currentConv = conv
	_ = r.persistConversation(context.Background())

	return conv
}

// SendMessage 发送消息到对话
func (r *ChatRouter) SendMessage(role, content string) (*Message, *TaskRequest, *WorkflowRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ensureDefaultConversation()

	if role == "" {
		role = "user"
	}
	// 添加用户消息
	userMsg := Message{
		Role:      role,
		Content:   content,
		Timestamp: metav1.Now(),
	}
	r.currentConv.Messages = append(r.currentConv.Messages, userMsg)

	// 限制历史长度
	if len(r.currentConv.Messages) > r.maxHistory {
		r.currentConv.Messages = r.currentConv.Messages[len(r.currentConv.Messages)-r.maxHistory:]
	}

	// 生成架构回复和可能的任务/工作流
	assistantMsg, taskRequest, workflowRequest := r.generateResponse(content)
	r.currentConv.Messages = append(r.currentConv.Messages, assistantMsg)
	r.currentConv.UpdatedAt = metav1.Now()

	// 如果有待批准的任务，加入任务队列
	if taskRequest != nil {
		r.addPendingTask(taskRequest)
		_ = r.persistPendingTasks(context.Background())
	}
	if workflowRequest != nil {
		r.addPendingWorkflow(workflowRequest)
	}

	_ = r.persistConversation(context.Background())
	return &assistantMsg, taskRequest, workflowRequest
}

// AddSystemMessage 添加系统消息到当前对话（用于任务反馈等）
func (r *ChatRouter) AddSystemMessage(content string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.ensureDefaultConversation()

	msg := Message{
		Role:      "system",
		Content:   content,
		Timestamp: metav1.Now(),
	}
	r.currentConv.Messages = append(r.currentConv.Messages, msg)
	r.currentConv.UpdatedAt = metav1.Now()
	_ = r.persistConversation(context.Background())
}

// generateResponse 生成架构的回复（使用 LLM，支持对话历史）
func (r *ChatRouter) generateResponse(userMessage string) (Message, *TaskRequest, *WorkflowRequest) {
	// 检查是否需要质量检查
	needsQualityCheck := containsKeywords(userMessage, []string{"检查", "质量", "验证", "审核", "审查", "quality"})

	// 检查是否是批准/拒绝指令
	if containsKeywords(userMessage, []string{"批准", "同意", "确认", "好的"}) {
		// 查找最近的待批准任务
		if len(r.pendingWorkflows) > 0 {
			lastWF := r.pendingWorkflows[len(r.pendingWorkflows)-1]
			wf, err := r.ApproveWorkflow(lastWF.ID, "user")
			if err != nil {
				return Message{Role: "assistant", Content: fmt.Sprintf("工作流批准失败: %v", err), Timestamp: metav1.Now()}, nil, nil
			}
			name := wf.CreatedName
			if name == "" {
				name = wf.ProposedName
			}
			content := fmt.Sprintf("✅ 工作流已批准并创建！\n\n名称: %s\n模板: %s\n描述: %s\n\n工作流正在执行，可在「工作流」页查看进度。", name, wf.Template, wf.Description)
			return Message{Role: "assistant", Content: content, Timestamp: metav1.Now()}, nil, nil
		}
		if len(r.pendingTasks) > 0 {
			lastTask := r.pendingTasks[len(r.pendingTasks)-1]
			lastTask.Approved = true
			now := metav1.Now()
			lastTask.ApprovedAt = &now
			lastTask.ApprovedBy = "user"

			// 从队列中移除
			r.pendingTasks = r.pendingTasks[:len(r.pendingTasks)-1]
			_ = r.persistPendingTasks(context.Background())

			// 创建 Task CRD
			if r.client != nil {
				taskCRD, err := r.CreateTaskFromRequest(lastTask)
				if err == nil {
					ctx := context.Background()
					if r.currentConv != nil && r.currentConv.Context != nil {
						ctx = r.currentConv.Context
					}
					if err := r.client.Create(ctx, taskCRD); err == nil {
						content := fmt.Sprintf("✅ 任务已批准并创建！\n\n任务名称: %s\n描述: %s\n\n任务正在执行中，请稍候查看结果。", taskCRD.Name, lastTask.Description)
						return Message{Role: "assistant", Content: content, Timestamp: metav1.Now()}, nil, nil
					}
				}
			}
			content := fmt.Sprintf("✅ 任务已批准！\n\n描述: %s\n\n任务正在执行中。", lastTask.Description)
			return Message{Role: "assistant", Content: content, Timestamp: metav1.Now()}, nil, nil
		}
		return Message{Role: "assistant", Content: "没有待批准的任务或工作流。请先描述您的需求。", Timestamp: metav1.Now()}, nil, nil
	}

	if containsKeywords(userMessage, []string{"拒绝", "取消", "不要"}) {
		if len(r.pendingTasks) > 0 {
			r.pendingTasks = r.pendingTasks[:len(r.pendingTasks)-1]
			_ = r.persistPendingTasks(context.Background())
			return Message{Role: "assistant", Content: "好的，已取消该任务。请告诉我您需要什么。", Timestamp: metav1.Now()}, nil, nil
		}
		if len(r.pendingWorkflows) > 0 {
			r.pendingWorkflows = r.pendingWorkflows[:len(r.pendingWorkflows)-1]
			return Message{Role: "assistant", Content: "好的，已取消该工作流。请告诉我您需要什么。", Timestamp: metav1.Now()}, nil, nil
		}
		return Message{Role: "assistant", Content: "没有待批准的任务或工作流。", Timestamp: metav1.Now()}, nil, nil
	}

	// 构建对话历史给 LLM
	historyContext := r.buildConversationHistory()

	// 使用 AI 生成回复
	if r.aiService != nil {
		systemPrompt := fmt.Sprintf(`你是一个智能任务编排助手。你的职责是：
1. 理解用户需求，区分「单步任务」与「多步工作流」
2. 单步、一次性任务：在回复末尾附加 [CREATE_TASK:任务描述]
3. 多章小说、大纲+逐章+合并：必须用工作流，在回复末尾附加：
   [CREATE_WORKFLOW:novel-outline-chapters:{"chapterCount":"100"}]
   历史/古装/朝代题材自动启用联网调研，可用：
   [CREATE_WORKFLOW:novel-team-historical:{"chapterCount":"20","historicalEra":"唐朝开元"}]
   可根据用户意图调整 chapterCount（如 20、50、100）
4. 用户说「批准」「同意」时，批准最近的待办（任务或工作流）
5. 用简洁中文回复；不要向用户展示方括号标记本身的格式说明

当前待批准任务: %d，待批准工作流: %d

小说工作流示例标记：
[CREATE_WORKFLOW:novel-outline-chapters:{"chapterCount":"100"}]`, len(r.pendingTasks), len(r.pendingWorkflows))

		userMsg := userMessage
		if historyContext != "" {
			userMsg = fmt.Sprintf("对话历史:\n%s\n\n用户新消息: %s", historyContext, userMessage)
		}

		resp, err := r.aiService.PlannerChat(context.Background(), systemPrompt, userMsg)
		if err == nil && resp != "" {
			var workflowRequest *WorkflowRequest
			resp, workflowRequest = parseWorkflowMarker(resp)
			if workflowRequest != nil {
				workflowRequest.Prompt = userMessage
			} else if workflowRequest = InferNovelWorkflowRequest(userMessage); workflowRequest != nil {
				resp = strings.TrimSpace(resp + "\n\n我已为您规划长篇小说工作流，请在侧栏或下方批准后开始执行。")
			}

			// 检查是否需要创建任务
			if strings.Contains(resp, "[CREATE_TASK:") {
				start := strings.Index(resp, "[CREATE_TASK:") + len("[CREATE_TASK:")
				end := strings.Index(resp[start:], "]")
				if end != -1 {
					taskDesc := strings.TrimSpace(resp[start : start+end])
					resp = strings.Replace(resp, fmt.Sprintf("[CREATE_TASK:%s]", taskDesc), "", 1)
					resp = strings.TrimSpace(resp)

					taskRequest := &TaskRequest{
						ID:          nextTaskID(r.pendingTasks),
						Description: taskDesc,
						Rationale:   "根据用户对话需求自动生成",
						Priority:    5,
						Resources: ResourceEstimate{
							CPU:      "500m",
							Memory:   "256Mi",
							Duration: 60,
						},
						NeedsQualityCheck: needsQualityCheck,
						CreatedAt:         metav1.Now(),
						Approved:          false,
					}
					return Message{Role: "assistant", Content: resp, Timestamp: metav1.Now()}, taskRequest, workflowRequest
				}
			}
			return Message{Role: "assistant", Content: resp, Timestamp: metav1.Now()}, nil, workflowRequest
		}
	}

	// 无 LLM 时仍可根据关键词推断小说工作流
	if workflowRequest := InferNovelWorkflowRequest(userMessage); workflowRequest != nil {
		content := fmt.Sprintf("我理解您要写小说。我已为您规划 %s 章长篇小说工作流，请在侧栏批准后开始执行。", workflowRequest.Params["chapterCount"])
		return Message{Role: "assistant", Content: content, Timestamp: metav1.Now()}, nil, workflowRequest
	}

	// 默认回复
	content := fmt.Sprintf("我理解您的需求：%s\n\n请告诉我您需要什么具体的任务，我会帮您规划和执行。", userMessage)
	return Message{Role: "assistant", Content: content, Timestamp: metav1.Now()}, nil, nil
}

// buildConversationHistory 构建对话历史上下文
func (r *ChatRouter) buildConversationHistory() string {
	if r.currentConv == nil || len(r.currentConv.Messages) == 0 {
		return ""
	}

	var history []string
	// 取最近 6 条消息作为上下文
	start := 0
	if len(r.currentConv.Messages) > 6 {
		start = len(r.currentConv.Messages) - 6
	}
	for _, msg := range r.currentConv.Messages[start:] {
		role := "用户"
		if msg.Role == "assistant" {
			role = "助手"
		} else if msg.Role == "system" {
			role = "系统"
		}
		history = append(history, fmt.Sprintf("%s: %s", role, msg.Content))
	}
	return strings.Join(history, "\n")
}

// addPendingTask 添加待批准任务到队列
func (r *ChatRouter) addPendingTask(task *TaskRequest) {
	r.taskQueueMu.Lock()
	defer r.taskQueueMu.Unlock()
	r.pendingTasks = append(r.pendingTasks, task)
}

// ListPendingTasks 列出待批准任务
func (r *ChatRouter) ListPendingTasks() []*TaskRequest {
	r.taskQueueMu.RLock()
	defer r.taskQueueMu.RUnlock()
	return r.pendingTasks
}

// ApproveTask 批准任务并创建实际的 Task CRD
func (r *ChatRouter) ApproveTask(taskID, approver string) (*TaskRequest, error) {
	r.taskQueueMu.Lock()
	defer r.taskQueueMu.Unlock()

	for i, task := range r.pendingTasks {
		if task.ID == taskID {
			task.Approved = true
			now := metav1.Now()
			task.ApprovedAt = &now
			task.ApprovedBy = approver

			// 从队列中移除
			r.pendingTasks = append(r.pendingTasks[:i], r.pendingTasks[i+1:]...)
			_ = r.persistPendingTasks(context.Background())

			// 创建实际的 Task CRD
			taskCRD, err := r.CreateTaskFromRequest(task)
			if err != nil {
				return task, fmt.Errorf("创建 Task CRD 失败：%w", err)
			}

			// 使用 k8s client 创建 Task
			if r.client != nil {
				ctx := r.currentConv.Context
				if ctx == nil {
					ctx = context.Background()
				}
				if err := r.client.Create(ctx, taskCRD); err != nil {
					return task, fmt.Errorf("创建 Task 资源失败：%w", err)
				}
				task.Description = fmt.Sprintf("%s (已创建 Task CRD: %s)", task.Description, taskCRD.Name)
			}

			return task, nil
		}
	}
	return nil, fmt.Errorf("任务不存在：%s", taskID)
}

// RejectTask 拒绝任务
func (r *ChatRouter) RejectTask(taskID, reason string) error {
	r.taskQueueMu.Lock()
	defer r.taskQueueMu.RUnlock()

	for i, task := range r.pendingTasks {
		if task.ID == taskID {
			task.RejectionReason = reason
			// 从队列中移除
			r.pendingTasks = append(r.pendingTasks[:i], r.pendingTasks[i+1:]...)
			_ = r.persistPendingTasks(context.Background())
			return nil
		}
	}
	return fmt.Errorf("任务不存在：%s", taskID)
}

// CreateTaskFromRequest 根据批准的任务创建实际 Task CRD
func (r *ChatRouter) CreateTaskFromRequest(task *TaskRequest) (*agentflowiov1alpha1.Task, error) {
	if !task.Approved {
		return nil, fmt.Errorf("任务未被批准")
	}

	taskCRD := &agentflowiov1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("chat-%s", task.ID),
			Namespace: "default",
			Labels: map[string]string{
				"agentflow.io/needs-quality-check": fmt.Sprintf("%v", task.NeedsQualityCheck),
			},
		},
		Spec: agentflowiov1alpha1.TaskSpec{
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{fmt.Sprintf("echo 'Executing: %s'", task.Description)},
			Image:   "docker.io/library/alpine:latest",
			RetryPolicy: agentflowiov1alpha1.RetryPolicy{
				MaxRetries:        3,
				RetryDelaySeconds: 10,
				RetryOn:           []agentflowiov1alpha1.TaskCondition{agentflowiov1alpha1.TaskConditionOnFailure},
			},
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					"cpu":    resource.MustParse(task.Resources.CPU),
					"memory": resource.MustParse(task.Resources.Memory),
				},
				Requests: corev1.ResourceList{
					"cpu":    resource.MustParse("250m"),
					"memory": resource.MustParse("128Mi"),
				},
			},
			TimeoutSeconds: func() *int32 { i := int32(task.Resources.Duration); return &i }(),
		},
	}

	return taskCRD, nil
}

// GetConversation 获取对话
func (r *ChatRouter) GetConversation(id string) *Conversation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.conversations[id]
}

// GetCurrentConversation 获取当前对话
func (r *ChatRouter) GetCurrentConversation() *Conversation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.currentConv
}

// 辅助函数
func containsKeywords(text string, keywords []string) bool {
	for _, kw := range keywords {
		if contains(text, kw) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
