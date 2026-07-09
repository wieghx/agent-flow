// Package flow implements agent orchestration workflows using the eino framework
package flow

import (
	"context"
	"fmt"
	"os"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/ai"
	sandboxv1beta1 "sigs.k8s.io/agent-sandbox/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/cloudwego/eino/compose"
)

// State 是流程的状态，在各节点间传递
type State struct {
	// 任务信息
	Task *agentflowiov1alpha1.Task
	// AgentSandbox 信息 (官方 agent-sandbox CRD)
	AgentSandbox *sandboxv1beta1.Sandbox
	// 执行结果
	ExecutionResult string
	ExecutionError  error
	// 流程上下文
	Context context.Context
	// Client for Kubernetes operations
	Client client.Client
	// AI 服务（Planner/Worker/Monitor 各自独立）
	AIService *ai.Service
	// 重试次数
	RetryCount int
	// 最大重试次数
	MaxRetries int
	// 当前阶段
	Phase string
	// Worker 执行指令（由 Planner 生成）
	WorkerInstruction string
	// Worker 产出物
	WorkerOutput string
	// 质量阈值（>= 此分数通过）
	QualityThreshold int
	// Monitor 反馈（失败时用于指导重试）
	MonitorFeedback string
	// MonitorEval 结构化评估结果
	MonitorEval *EvalResult
	// MonitorTaskType overrides auto-detected rubric (e.g. novel-chapter)
	MonitorTaskType string
	// ConsistencyContext carries cross-chapter reference for workflow chapters
	ConsistencyContext string
	// MonitorTier is light or full (workflow chapter QC).
	MonitorTier string
	// TeamMode enables polish + multi-gate QC for novel team pipeline.
	TeamMode bool
	// TokenUsage accumulates LLM tokens consumed in this flow run.
	TokenUsage ai.TokenUsage
}

// AgentFlow 是基于 eino 框架构建的 Agent 编排流程
// 使用 eino 的 Chain 和 Graph 概念来组织任务分配、执行和监控
type AgentFlow struct {
	// 使用的 eino Chain
	chain *compose.Chain[State, State]
	// 缓存的编译结果
	compiledRunnable compose.Runnable[State, State]
}

// NewAgentFlow 创建新的 eino 流程
func NewAgentFlow() *AgentFlow {
	return &AgentFlow{
		chain: compose.NewChain[State, State](),
	}
}

// AddNode 添加节点到流程
func (f *AgentFlow) AddNode(node Node) *AgentFlow {
	// 将自定义 Node 转换为 eino 的 Lambda 节点
	lambda := compose.InvokableLambda(node.Run)
	f.chain.AppendLambda(lambda)
	return f
}

// Build 构建流程（返回自身）
func (f *AgentFlow) Build() *AgentFlow {
	return f
}

// Compile 编译流程为可执行的 Runnable
func (f *AgentFlow) Compile() error {
	ctx := context.Background()
	runnable, err := f.chain.Compile(ctx)
	if err != nil {
		return err
	}
	f.compiledRunnable = runnable
	return nil
}

// Execute 执行流程
func (f *AgentFlow) Execute(ctx context.Context, input State) (*State, error) {
	if f.compiledRunnable == nil {
		return nil, ErrFlowNotCompiled
	}

	output, err := f.compiledRunnable.Invoke(ctx, input)
	if err != nil {
		return nil, err
	}

	return &output, nil
}

// ErrFlowNotCompiled is returned when Execute is called before Compile
var ErrFlowNotCompiled = &FlowError{"flow not compiled, call Compile() first"}

// FlowError represents an error in the flow execution
type FlowError struct {
	Message string
}

func (e *FlowError) Error() string {
	return e.Message
}

// Node 是 eino 节点的抽象接口
// 每个节点实现 Run 方法来处理状态转换
type Node interface {
	// Run 执行节点逻辑，接收输入状态并返回处理后的状态
	Run(ctx context.Context, input State) (State, error)
}

// PlannerNode 是任务规划节点（大脑）
type PlannerNode struct {
	Name string
}

// Run 执行规划逻辑 - 创建官方 agent-sandbox
func (n *PlannerNode) Run(ctx context.Context, input State) (State, error) {
	input.Phase = "planning"
	input.ExecutionResult = "任务规划完成"

	logger := log.FromContext(ctx).WithName("planner-node")
	logger.Info("starting planning",
		"namespace", input.Task.Namespace,
		"task", input.Task.Name,
		"phase", input.Task.Status.Phase,
	)

	// 先检查官方 agent-sandbox 是否已存在
	sandboxName := fmt.Sprintf("%s-sandbox", input.Task.Name)
	logger.V(1).Info("looking for existing sandbox", "namespace", input.Task.Namespace, "sandbox", sandboxName)
	existingSandbox := &sandboxv1beta1.Sandbox{}
	existingKey := types.NamespacedName{
		Name:      sandboxName,
		Namespace: input.Task.Namespace,
	}
	// 使用 directGet 获取最新数据，避免缓存
	err := input.Client.Get(ctx, existingKey, existingSandbox)
	if err != nil {
		logger.V(1).Info("sandbox not found, will create", "error", err)
	} else {
		logger.Info("sandbox already exists",
			"sandbox", sandboxName,
			"uid", existingSandbox.UID,
			"resourceVersion", existingSandbox.ResourceVersion,
		)
		input.AgentSandbox = existingSandbox
		input.ExecutionResult = fmt.Sprintf("Sandbox already exists: %s", sandboxName)
		return input, nil
	}

	// 构建官方 agent-sandbox Sandbox 资源
	agentSandbox := &sandboxv1beta1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sandboxName,
			Namespace: input.Task.Namespace,
			Labels: map[string]string{
				"agents.x-k8s.io/sandbox":   sandboxName,
				"agentflow.io/task":         input.Task.Name,
				"agentflow.io/sandbox-kind": "agent-sandbox",
			},
			Annotations: map[string]string{
				"agentflow.io/task-uid": string(input.Task.UID),
			},
		},
		Spec: sandboxv1beta1.SandboxSpec{
			PodTemplate: sandboxv1beta1.PodTemplate{
				Spec: corev1.PodSpec{
					RestartPolicy:    corev1.RestartPolicyNever,
					RuntimeClassName: input.Task.Spec.RuntimeClassName,
					SecurityContext:  input.Task.Spec.PodSecurityContext,
					Containers: []corev1.Container{
						{
							Name:    input.Task.Name,
							Image:   input.Task.Spec.Image,
							Command: input.Task.Spec.Command,
							Args:    input.Task.Spec.Args,
							Env:     input.Task.Spec.Env,
							Resources: corev1.ResourceRequirements{
								Limits: input.Task.Spec.Resources.Limits,
							},
						},
					},
				},
				ObjectMeta: sandboxv1beta1.PodMetadata{
					Labels: map[string]string{
						"agents.x-k8s.io/sandbox": input.Task.Name,
					},
				},
			},
			Lifecycle: sandboxv1beta1.Lifecycle{
				ShutdownTime:   nil,
				ShutdownPolicy: nil,
			},
			OperatingMode: sandboxv1beta1.SandboxOperatingModeRunning,
		},
	}

	// 创建官方 agent-sandbox
	logger.Info("creating sandbox", "sandbox", sandboxName)
	if err := input.Client.Create(ctx, agentSandbox); err != nil {
		logger.Error(err, "failed to create sandbox", "sandbox", sandboxName)
		return input, fmt.Errorf("创建 AgentSandbox 失败：%w", err)
	}

	input.AgentSandbox = agentSandbox
	input.ExecutionResult = fmt.Sprintf("AgentSandbox 已创建：%s (由官方 agent-sandbox controller 执行)", sandboxName)
	logger.Info("sandbox created", "sandbox", sandboxName)

	return input, nil
}

// ClearTaskPVCOutput removes stale sandbox/worker cache so step retries regenerate prose.
func ClearTaskPVCOutput(namespace, taskName string) {
	outputFile := fmt.Sprintf("/data/outputs/%s/%s.txt", namespace, taskName)
	_ = os.Remove(outputFile)
}

func shouldBypassPVCOutput(input State) bool {
	if ShouldUseSegmentedChapter(input.WorkerInstruction, input.MonitorTaskType) {
		return true
	}
	taskType := input.MonitorTaskType
	if taskType == "" {
		taskType = DetectTaskType(input.WorkerInstruction)
	}
	if taskType == TaskTypeNovelChapterTeam {
		return true
	}
	if input.MonitorFeedback != "" || input.RetryCount > 0 {
		return true
	}
	return false
}

func pvcChapterOutputTooShort(input State, content string) bool {
	taskType := input.MonitorTaskType
	if taskType == "" {
		taskType = DetectTaskType(input.WorkerInstruction)
	}
	if taskType != TaskTypeNovelChapter && taskType != TaskTypeNovelChapterTeam {
		return false
	}
	target := ParseTargetWordsFromInstruction(input.WorkerInstruction)
	if target <= 0 {
		return false
	}
	return len([]rune(ExtractChineseProse(content))) < MinChapterRunes(target)/2
}

func buildWorkerSystemPrompt(instruction string) string {
	return buildWorkerSystemPromptFor(instruction, "")
}

func buildWorkerSystemPromptFor(instruction, monitorTaskType string) string {
	taskType := monitorTaskType
	if taskType == "" {
		taskType = DetectTaskType(instruction)
	}
	switch taskType {
	case TaskTypeNovelOutline:
		return `你是小说策划编辑。根据指令生成小说大纲，严格只输出一个 JSON 对象。
要求：
1. 不要输出思考过程、分析、英文备注或 markdown 代码块
2. JSON 必须含 title、synopsis、characters、chapters 字段且可被解析
3. chapters 数组每项含 num、title、summary，num 从 1 连续递增
4. 直接以 { 开头、以 } 结尾`
	case TaskTypeNovelOutlineRefine:
		return `你是资深小说策划编辑。根据指令对现有大纲进行精修，严格只输出一个 JSON 对象。
要求：
1. 不要输出思考过程、分析、英文备注或 markdown 代码块
2. 读取 outline.json 改进，保留好的部分；outline-draft.json 为初稿备份仅供对照
3. JSON 必须含 title、synopsis、characters、chapters 字段且可被解析
4. 重点改进：主线完整性、冲突递进、人物弧光、节奏收束、伏笔回收；不得删减或合并章节
5. 直接以 { 开头、以 } 结尾`
	case TaskTypeNovelPlot:
		return `你是小说剧情编剧。根据梗概扩写剧情脚本，只输出剧情脚本文本。
要求：
1. 不要输出思考过程或 markdown 代码块
2. 包含场景节拍、冲突、对话要点、衔接与悬念
3. 不要写成完整散文正文`
	case TaskTypeNovelOutlineSkeleton:
		return `你是小说策划编辑。根据指令生成长篇分卷骨架，严格只输出一个 JSON 对象。
要求：
1. 不要输出思考过程、分析、英文备注或 markdown 代码块
2. JSON 必须含 title、synopsis、characters、volumes 字段且可被解析
3. volumes 每项含 num、title、startChapter、endChapter、theme、summary，章节范围连续无遗漏
4. 直接以 { 开头、以 } 结尾`
	case TaskTypeNovelChapter:
		target := ParseTargetWordsFromInstruction(instruction)
		minRunes := MinChapterRunes(target)
		lengthHint := ""
		if target > 0 {
			lengthHint = fmt.Sprintf("5. 本章正文不少于 %d 字（目标约 %d 字），写完整场景与对话，不要草草收尾\n6. 必须以句号、问号或感叹号等完整收束，不要中途截断", minRunes, target)
		} else {
			lengthHint = "5. 正文需充实完整，以完整句子收束，不要中途截断"
		}
		return fmt.Sprintf(`你是小说作者。根据大纲与上下文撰写本章正文。
要求：
1. 不要输出思考过程或写作说明
2. 直接输出中文小说正文，自然衔接上一章
3. 人物、时间线、伏笔与设定保持一致；严禁更换主角姓名或引入未在大纲登记的主要角色
4. 全章保持统一叙事人称、语体与节奏；若分多段撰写，段与段须无缝衔接，不得像拼凑的独立片段
%s`, lengthHint)
	default:
		return "你是一个专业的任务执行者。根据给定的指令执行任务，生成高质量、完整的产出物。要求：\n1. 内容丰富、有深度\n2. 格式规范、排版清晰\n3. 直接输出最终结果，不要输出过程说明"
	}
}

// WorkerNode 是 AI 执行节点（执行者）
// 调用 AI 执行任务，生成文本产出物
type WorkerNode struct {
	Name string
}

// Run 执行任务逻辑 — 优先读取 Sandbox 产出，否则调用 AI 生成
func (n *WorkerNode) Run(ctx context.Context, input State) (State, error) {
	input.Phase = "executing"
	logger := log.FromContext(ctx).WithName("worker-node")

	// 优先从 PVC 读取 Sandbox 的产出物（章节分段模式或明显过短时不复用缓存）
	if input.Task != nil && !shouldBypassPVCOutput(input) {
		outputFile := fmt.Sprintf("/data/outputs/%s/%s.txt", input.Task.Namespace, input.Task.Name)
		if data, err := os.ReadFile(outputFile); err == nil && len(data) > 10 {
			content := strings.TrimSpace(string(data))
			// 跳过垃圾产出（包含未完成 JSON 或过短的内容）
			if !strings.Contains(content, "```") && !strings.HasPrefix(content, "{") && !pvcChapterOutputTooShort(input, content) {
				input.WorkerOutput = content
				input.ExecutionResult = fmt.Sprintf("Worker 从 Sandbox 产出物读取完成，长度: %d 字符", len(input.WorkerOutput))
				logger.Info("loaded output from PVC", "path", outputFile, "bytes", len(content))
				return input, nil
			}
			logger.Info("PVC output quality too low, regenerating with AI", "path", outputFile)
		}
	}

	// PVC 中没有产出物，调用 AI 生成
	if input.AIService == nil {
		return input, fmt.Errorf("AI 服务未初始化")
	}

	var output string
	var err error
	if ShouldUseSegmentedChapter(input.WorkerInstruction, input.MonitorTaskType) {
		logger.Info("writing chapter in segmented mode")
		var segUsage ai.TokenUsage
		output, segUsage, err = GenerateSegmentedChapter(ctx, input.AIService, input.WorkerInstruction, input.MonitorFeedback)
		if err != nil {
			return input, fmt.Errorf("worker 分段撰写失败：%w", err)
		}
		input.TokenUsage.Add(segUsage)
	} else {
		systemPrompt := buildWorkerSystemPromptFor(input.WorkerInstruction, input.MonitorTaskType)
		userMessage := input.WorkerInstruction
		logger.V(1).Info("received worker instruction", "instruction", userMessage)
		if input.MonitorFeedback != "" {
			userMessage = fmt.Sprintf("%s\n\n上次执行的反馈（请根据反馈改进）：%s", input.WorkerInstruction, input.MonitorFeedback)
		}

		result, err := input.AIService.WorkerChat(ctx, systemPrompt, userMessage)
		if err != nil {
			return input, fmt.Errorf("worker AI 调用失败：%w", err)
		}
		input.TokenUsage.Add(result.Usage)
		output = result.Content

		output = NormalizeWorkerOutput(userMessage, output)
		if strings.Contains(userMessage, "小说作者") || strings.Contains(userMessage, "章节正文") {
			output = ExtractChineseProse(output)
		}
	}
	input.WorkerOutput = output
	input.ExecutionResult = fmt.Sprintf("Worker AI 执行完成，产出长度: %d 字符", len(output))

	// 保存产出物到 PVC / 本地目录
	if input.Task != nil {
		outputDir := os.Getenv("AGENTFLOW_OUTPUT_DIR")
		if outputDir == "" {
			outputDir = "/data/outputs"
		}
		taskDir := fmt.Sprintf("%s/%s", outputDir, input.Task.Namespace)
		if err := os.MkdirAll(taskDir, 0755); err != nil {
			logger.Error(err, "failed to create output dir", "path", taskDir)
		}
		outputFile := fmt.Sprintf("%s/%s.txt", taskDir, input.Task.Name)
		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			logger.Error(err, "failed to save output", "path", outputFile)
		} else {
			logger.Info("output saved", "path", outputFile, "bytes", len(output))
		}
	}

	return input, nil
}

// MonitorNode 是 AI 质量检查节点（监工）
// 调用 AI 评估产出质量，返回评分和反馈
type MonitorNode struct {
	Name string
}

// Run 执行质量检查逻辑 — 规则预检 + AI 评估
func (n *MonitorNode) Run(ctx context.Context, input State) (State, error) {
	input.Phase = "monitoring"

	if input.AIService == nil {
		return input, fmt.Errorf("AI 服务未初始化")
	}

	configPrompt := ""
	if cfg := input.AIService.Config(); cfg != nil {
		configPrompt = cfg.Monitor.SystemPrompt
	}

	tier := input.MonitorTier
	if tier == "" {
		tier = MonitorTierFull
	}
	eval, err := RunMonitorEvaluation(ctx, input.AIService, input.WorkerInstruction, input.WorkerOutput, input.QualityThreshold, input.RetryCount, configPrompt, input.MonitorFeedback, input.MonitorTaskType, input.ConsistencyContext, tier, input.TeamMode)
	if err != nil {
		return input, fmt.Errorf("monitor 评估失败：%w", err)
	}

	input.TokenUsage.Add(eval.TokenUsage)
	input.MonitorEval = eval
	if !eval.Passed {
		input.MonitorFeedback = FormatRetryFeedback(eval, input.QualityThreshold)
	} else {
		input.MonitorFeedback = eval.Feedback
	}
	input.ExecutionResult = fmt.Sprintf("Monitor 评估完成: score=%d, passed=%v, method=%s, type=%s",
		eval.Score, eval.Passed, eval.CheckMethod, eval.TaskType)

	return input, nil
}
