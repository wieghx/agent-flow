package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/flow"
	applog "agent-flow/internal/log"
	"agent-flow/internal/store"
	"agent-flow/internal/utils/safepath"
	wfengine "agent-flow/internal/workflow"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// API 聊天 API 服务器
type API struct {
	router     *flow.ChatRouter
	mux        *http.ServeMux
	client     client.Client
	novelStore store.Store
}

// NewAPI 创建新的 API 服务器
func NewAPI(router *flow.ChatRouter, k8sClient client.Client, novelStore store.Store) *API {
	api := &API{
		router:     router,
		mux:        http.NewServeMux(),
		client:     k8sClient,
		novelStore: novelStore,
	}
	api.setupRoutes()
	return api
}

// corsHandler 创建带 CORS 的 handler
func corsHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// setupRoutes 设置路由 - 使用统一的 CORS handler 包装整个 mux
func (a *API) setupRoutes() {
	// 直接注册 handler
	a.mux.HandleFunc("/", a.handleIndex)
	a.mux.HandleFunc("/chat", a.handleChat)
	a.mux.HandleFunc("/tasks/pending", a.handlePendingTasks)
	a.mux.HandleFunc("/tasks/approve", a.handleApproveTask)
	a.mux.HandleFunc("/tasks/reject", a.handleRejectTask)
	a.mux.HandleFunc("/conversation", a.handleConversation)
	a.mux.HandleFunc("/tasks", a.handleTasks)
	a.mux.HandleFunc("/tasks/create", a.handleCreateTask)
	a.mux.HandleFunc("/tasks/", a.handleTaskDetail)
	a.mux.HandleFunc("/tasks/events", a.handleTaskEvents)
	a.mux.HandleFunc("/tasks/evals", a.handleTaskEvals)
	a.mux.HandleFunc("/workflows/pending", a.handlePendingWorkflows)
	a.mux.HandleFunc("/workflows/approve", a.handleApproveWorkflow)
	a.mux.HandleFunc("/workflows/reject", a.handleRejectWorkflow)
	a.mux.HandleFunc("/workflows/create", a.handleCreateWorkflow)
	a.mux.HandleFunc("/workflows/", a.handleWorkflowDetail)
	a.mux.HandleFunc("/workflows", a.handleWorkflows)
	a.mux.HandleFunc("/novels/create", a.handleCreateNovel)
	a.mux.HandleFunc("/novels/import", a.handleImportNovel)
	a.mux.HandleFunc("/novels/tokens/report", a.handleTokenReport)
	a.mux.HandleFunc("/novels/", a.handleNovelRoutes)
	a.mux.HandleFunc("/novels", a.handleNovelList)
	a.mux.HandleFunc("/outputs/", a.handleOutputFile)
	a.mux.HandleFunc("/observability", a.handleObservability)
}

// ServeHTTP 实现 http.Handler 接口 - 使用 corsHandler 包装整个 mux
func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	corsHandler(a.mux).ServeHTTP(w, r)
}

// Response 通用响应结构
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// --- 路由处理器 ---

// handleIndex 首页
func (a *API) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Message: "Agent Flow Chat API",
		Data: map[string]string{
			"endpoints": "/chat, /tasks, /workflows, /conversation",
		},
	})
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Message string `json:"message"`
	Role    string `json:"role,omitempty"` // "user", "system"
}

// handleChat 处理聊天消息
func (a *API) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	assistantMsg, taskRequest, workflowRequest := a.router.SendMessage(req.Role, req.Message)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Message: "Message processed",
		Data: map[string]interface{}{
			"assistant_reply":    assistantMsg.Content,
			"task_suggested":     taskRequest != nil,
			"task":               taskRequest,
			"workflow_suggested": workflowRequest != nil,
			"workflow":           workflowRequest,
		},
	})
}

// handlePendingTasks 获取待批准任务
func (a *API) handlePendingTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tasks := a.router.ListPendingTasks()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]interface{}{
			"count": len(tasks),
			"tasks": tasks,
		},
	})
}

// ApproveRequest 批准请求
type ApproveRequest struct {
	TaskID   string `json:"task_id"`
	Approver string `json:"approver"`
}

// handleApproveTask 批准任务
func (a *API) handleApproveTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ApproveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	task, err := a.router.ApproveTask(req.TaskID, req.Approver)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Message: "Task approved",
		Data:    task,
	})
}

// RejectRequest 拒绝请求
type RejectRequest struct {
	TaskID string `json:"task_id"`
	Reason string `json:"reason"`
}

// handleRejectTask 拒绝任务
func (a *API) handleRejectTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RejectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if err := a.router.RejectTask(req.TaskID, req.Reason); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Message: "Task rejected",
	})
}

// ConversationResponse 对话响应
type ConversationResponse struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Rules     string         `json:"rules"`
	Messages  []flow.Message `json:"messages"`
	CreatedAt string         `json:"created_at"`
	UpdatedAt string         `json:"updated_at"`
}

// handleConversation 获取当前对话
func (a *API) handleConversation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conv := a.router.GetCurrentConversation()
	if conv == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "No active conversation",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: ConversationResponse{
			ID:        conv.ID,
			Name:      conv.Name,
			Rules:     conv.Rules,
			Messages:  conv.Messages,
			CreatedAt: conv.CreatedAt.String(),
			UpdatedAt: conv.UpdatedAt.String(),
		},
	})
}

// TaskListResponse 任务列表响应
type TaskListResponse struct {
	Name         string                            `json:"name"`
	Namespace    string                            `json:"namespace"`
	Workflow     string                            `json:"workflow,omitempty"`
	StepID       string                            `json:"step_id,omitempty"`
	Phase        string                            `json:"phase"`
	Message      string                            `json:"message"`
	Output       string                            `json:"output"`
	Score        int32                             `json:"score"`
	Passed       bool                              `json:"passed"`
	Retries      int32                             `json:"retries"`
	QualityCheck *agentflowiov1alpha1.QualityCheck `json:"qualityCheck,omitempty"`
	CreatedAt    string                            `json:"created_at"`
	CompletionAt string                            `json:"completion_at,omitempty"`
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	Name        string                       `json:"name"`
	Namespace   string                       `json:"namespace"`
	Image       string                       `json:"image"`
	Command     []string                     `json:"command"`
	Args        []string                     `json:"args"`
	Env         []map[string]string          `json:"env,omitempty"`
	Resources   map[string]map[string]string `json:"resources"`
	RetryPolicy map[string]int               `json:"retryPolicy"`
}

// handleTasks 获取所有任务列表
func (a *API) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filterWorkflow := strings.TrimSpace(r.URL.Query().Get("workflow"))
	filterNamespace := strings.TrimSpace(r.URL.Query().Get("namespace"))

	listOpts := []client.ListOption{}
	if filterWorkflow != "" {
		listOpts = append(listOpts, client.MatchingLabels{"agentflow.io/workflow": filterWorkflow})
	}
	if filterNamespace != "" {
		listOpts = append(listOpts, client.InNamespace(filterNamespace))
	}

	taskList := &agentflowiov1alpha1.TaskList{}
	if err := a.client.List(r.Context(), taskList, listOpts...); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	tasks := make([]TaskListResponse, 0, len(taskList.Items))
	for _, task := range taskList.Items {
		output := ""
		if task.Status.Output != nil {
			output = task.Status.Output.Content
			if len(output) > 500 {
				output = output[:500] + "..."
			}
		}
		score := int32(0)
		passed := false
		if task.Status.QualityCheck != nil {
			score = task.Status.QualityCheck.Score
			passed = task.Status.QualityCheck.Passed
		}
		completionAt := ""
		if task.Status.CompletionTime != nil {
			completionAt = task.Status.CompletionTime.Time.Format("2006-01-02 15:04:05")
		}
		workflowName := ""
		stepID := ""
		if task.Labels != nil {
			workflowName = task.Labels["agentflow.io/workflow"]
			stepID = task.Labels["agentflow.io/workflow-step"]
		}
		tasks = append(tasks, TaskListResponse{
			Name:         task.Name,
			Namespace:    task.Namespace,
			Workflow:     workflowName,
			StepID:       stepID,
			Phase:        string(task.Status.Phase),
			Message:      task.Status.Message,
			Output:       output,
			Score:        score,
			Passed:       passed,
			Retries:      task.Status.Retries,
			QualityCheck: task.Status.QualityCheck,
			CreatedAt:    task.CreationTimestamp.Time.Format("2006-01-02 15:04:05"),
			CompletionAt: completionAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]interface{}{
			"count": len(tasks),
			"tasks": tasks,
		},
	})
}

// handleCreateTask 创建新任务
func (a *API) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// Build environment variables
	envVars := make([]corev1.EnvVar, 0, len(req.Env))
	for _, e := range req.Env {
		name := e["name"]
		value := e["value"]
		if name != "" {
			envVars = append(envVars, corev1.EnvVar{Name: name, Value: value})
		}
	}

	// Build resource limits
	limits := make(corev1.ResourceList)
	if cpu, ok := req.Resources["limits"]["cpu"]; ok {
		limits[corev1.ResourceCPU] = resource.MustParse(cpu)
	}
	if memory, ok := req.Resources["limits"]["memory"]; ok {
		limits[corev1.ResourceMemory] = resource.MustParse(memory)
	}

	// Create Task resource
	task := &agentflowiov1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
		Spec: agentflowiov1alpha1.TaskSpec{
			Image:   req.Image,
			Command: req.Command,
			Args:    req.Args,
			Env:     envVars,
			Resources: corev1.ResourceRequirements{
				Limits: limits,
			},
			RetryPolicy: agentflowiov1alpha1.RetryPolicy{
				MaxRetries: int32(req.RetryPolicy["maxRetries"]),
			},
		},
	}

	if err := a.client.Create(r.Context(), task); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Message: fmt.Sprintf("Task %s created", req.Name),
		Data:    task,
	})
}

// handleTaskDetail 处理单个任务的详细操作（GET: 日志，DELETE: 删除）
func (a *API) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	// Extract task name from path: /tasks/{name}
	path := strings.TrimPrefix(r.URL.Path, "/tasks/")
	taskName := strings.Split(path, "/")[0]

	if taskName == "" {
		http.Error(w, "Task name required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		a.handleTaskLogs(w, r, taskName)
	case http.MethodDelete:
		a.handleDeleteTask(w, r, taskName)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTaskLogs 获取任务日志
func (a *API) handleTaskLogs(w http.ResponseWriter, r *http.Request, taskName string) {
	// This is a simplified version - in production, you would need to:
	// 1. Get the Task to find the associated Sandbox
	// 2. Get the Sandbox to find the Pod
	// 3. Get the Pod logs from the cluster

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Error:   "Log retrieval requires pod access. Use kubectl logs directly.",
	})
}

// handleDeleteTask 删除任务
func (a *API) handleDeleteTask(w http.ResponseWriter, r *http.Request, taskName string) {
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}

	task := &agentflowiov1alpha1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name:      taskName,
			Namespace: namespace,
		},
	}

	if err := a.client.Delete(r.Context(), task); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Message: fmt.Sprintf("Task %s/%s deleted", namespace, taskName),
	})
}

// handleTaskEvals returns monitor evaluation history from Redis/memory store.
func (a *API) handleTaskEvals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}
	taskName := r.URL.Query().Get("name")
	if taskName == "" {
		http.Error(w, "name query parameter is required", http.StatusBadRequest)
		return
	}

	store := a.router.StateStore()
	if store == nil {
		http.Error(w, "state store unavailable", http.StatusServiceUnavailable)
		return
	}

	records, err := store.ListEvalHistory(r.Context(), namespace, taskName)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]interface{}{
			"count": len(records),
			"evals": records,
		},
	})
}

// handleTaskEvents streams task progress events via SSE.
// GET /tasks/events?namespace=default&name=chat-task-1
func (a *API) handleTaskEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		namespace = "default"
	}
	taskName := r.URL.Query().Get("name")
	if taskName == "" {
		http.Error(w, "name query parameter is required", http.StatusBadRequest)
		return
	}

	store := a.router.StateStore()
	if store == nil {
		http.Error(w, "state store unavailable", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ctx := r.Context()
	events, cancel, err := store.SubscribeEvents(ctx, namespace, taskName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cancel()

	fmt.Fprintf(w, "event: connected\ndata: {\"task\":\"%s\",\"namespace\":\"%s\"}\n\n", taskName, namespace)
	flusher.Flush()

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		case event, ok := <-events:
			if !ok {
				return
			}
			data, err := json.Marshal(event)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Step, data)
			flusher.Flush()
		}
	}
}

// WorkflowListResponse summarizes a workflow for API listing.
type WorkflowListResponse struct {
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Phase        string `json:"phase"`
	CurrentStep  string `json:"currentStep"`
	Progress     int32  `json:"progress"`
	Message      string `json:"message"`
	Template     string `json:"template,omitempty"`
	Workspace    string `json:"workspacePath,omitempty"`
	CreatedAt    string `json:"created_at"`
	CompletionAt string `json:"completion_at,omitempty"`
}

// CreateWorkflowRequest creates a workflow directly.
type CreateWorkflowRequest struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Prompt    string            `json:"prompt"`
	Template  string            `json:"template"`
	Params    map[string]string `json:"params"`
}

func (a *API) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	wfList := &agentflowiov1alpha1.WorkflowList{}
	if err := a.client.List(r.Context(), wfList); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
		return
	}

	workflows := make([]WorkflowListResponse, 0, len(wfList.Items))
	for _, wf := range wfList.Items {
		completionAt := ""
		if wf.Status.CompletionTime != nil {
			completionAt = wf.Status.CompletionTime.Time.Format("2006-01-02 15:04:05")
		}
		workflows = append(workflows, WorkflowListResponse{
			Name:         wf.Name,
			Namespace:    wf.Namespace,
			Phase:        string(wf.Status.Phase),
			CurrentStep:  wf.Status.CurrentStep,
			Progress:     wf.Status.Progress.Percent,
			Message:      wf.Status.Message,
			Template:     wf.Spec.Template,
			Workspace:    wf.Status.WorkspacePath,
			CreatedAt:    wf.CreationTimestamp.Time.Format("2006-01-02 15:04:05"),
			CompletionAt: completionAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]interface{}{
			"count":     len(workflows),
			"workflows": workflows,
		},
	})
}

func (a *API) handleWorkflowDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/workflows/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var namespace, name string
	switch len(parts) {
	case 1:
		name = parts[0]
		namespace = r.URL.Query().Get("namespace")
		if namespace == "" {
			namespace = "default"
		}
	case 2:
		namespace, name = parts[0], parts[1]
	default:
		http.Error(w, "Workflow path must be /workflows/{name} or /workflows/{namespace}/{name}", http.StatusBadRequest)
		return
	}
	if name == "" || name == "pending" || name == "approve" || name == "reject" || name == "create" {
		http.Error(w, "Workflow name required", http.StatusBadRequest)
		return
	}

	wf := &agentflowiov1alpha1.Workflow{}
	if err := a.client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, wf); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{Success: true, Data: wf})
}

func (a *API) handlePendingWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	workflows := a.router.ListPendingWorkflows()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Data: map[string]interface{}{
			"count":     len(workflows),
			"workflows": workflows,
		},
	})
}

type ApproveWorkflowRequest struct {
	WorkflowID string `json:"workflow_id"`
	Approver   string `json:"approver"`
}

func (a *API) handleApproveWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ApproveWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
		return
	}

	wf, err := a.router.ApproveWorkflow(req.WorkflowID, req.Approver)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Message: "Workflow approved",
		Data:    wf,
	})
}

type RejectWorkflowRequest struct {
	WorkflowID string `json:"workflow_id"`
	Reason     string `json:"reason"`
}

func (a *API) handleRejectWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RejectWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
		return
	}

	if err := a.router.RejectWorkflow(req.WorkflowID, req.Reason); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{Success: true, Message: "Workflow rejected"})
}

func (a *API) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateWorkflowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
		return
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = "default"
	}
	template := req.Template
	if template == "" {
		template = wfengine.DefaultNovelTemplate(req.Params, req.Prompt)
	}
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("wf-%d", time.Now().Unix())
	}

	chapterCount := wfengine.IntParam(req.Params, "chapterCount", 10)
	defaults := wfengine.DefaultNovelParams(chapterCount)
	req.Params = wfengine.MergeParams(defaults, req.Params)

	wf, err := flow.NewWorkflowCRD(template, req.Prompt, req.Params, name, namespace)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
		return
	}
	if err := a.client.Create(r.Context(), wf); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(Response{
		Success: true,
		Message: fmt.Sprintf("Workflow %s created", name),
		Data:    wf,
	})
}

// handleOutputFile 从 PVC 提供产出物文件下载
func (a *API) handleOutputFile(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/outputs/")
	filePath, err := safepath.ResolveUnderRoot("/data/outputs", rel)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{Success: false, Error: err.Error()})
		return
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Response{
			Success: false,
			Error:   "文件不存在: " + filePath,
		})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(filePath)))
	w.Write(data)
}

// StartServer 启动 API 服务器
func (a *API) StartServer(port int) error {
	addr := fmt.Sprintf(":%d", port)
	applog.Component("api").Info("starting HTTP API server",
		"addr", addr,
		"endpoints", []string{
			"GET /",
			"POST /chat",
			"GET /tasks/pending",
			"POST /tasks/approve",
			"POST /tasks/reject",
			"GET /conversation",
			"GET /tasks",
			"POST /tasks/create",
			"GET /tasks/{name}/logs",
			"GET /tasks/events",
			"GET /tasks/evals",
			"DELETE /tasks/{name}",
			"GET /workflows",
			"POST /workflows/create",
			"GET /workflows/{name}",
			"GET /workflows/pending",
			"POST /workflows/approve",
			"POST /workflows/reject",
		},
	)
	return http.ListenAndServe(addr, a)
}
