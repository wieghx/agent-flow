package architecture

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/ai"
	"agent-flow/internal/cache"
	"agent-flow/internal/config"
	"agent-flow/internal/flow"
	applog "agent-flow/internal/log"
	"agent-flow/internal/metrics"
	retryutil "agent-flow/internal/retry"
	wfengine "agent-flow/internal/workflow"
	sandboxv1beta1 "sigs.k8s.io/agent-sandbox/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const defaultTaskConcurrentReconciles = 8

// TaskPlannerEino 是基于 eino 框架的任务分配器（大脑）
type TaskPlannerEino struct {
	client.Client
	Scheme        *runtime.Scheme
	AIService     *ai.Service
	ChatRouter    *flow.ChatRouter
	Store         cache.StateStore
	Retry         config.RetryConfig
	Quality       config.QualityConfig
	networkConfig *SandboxNetworkConfig
}

// +kubebuilder:rbac:groups=agentflow.io,resources=tasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agentflow.io,resources=tasks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agentflow.io,resources=tasks/finalizers,verbs=update
// +kubebuilder:rbac:groups=agents.x-k8s.io,resources=sandboxes,verbs=create;update;patch;delete
// +kubebuilder:rbac:groups=agents.x-k8s.io,resources=sandboxes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=create;update;patch;delete;get;list;watch

// Reconcile 使用 eino 流程来协调任务处理
func (p *TaskPlannerEino) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// 获取 Task 实例
	task := &agentflowiov1alpha1.Task{}
	if err := p.Get(ctx, req.NamespacedName, task); err != nil {
		if k8serrors.IsNotFound(err) {
			// Task 已被删除（常见于清理残留 CRD）；队列中可能仍有陈旧事件，无需记录 INFO。
			logger.V(1).Info("task not found, skipping reconcile", "task", req.Name, "namespace", req.Namespace)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("获取任务失败：%w", err)
	}

	logger.Info("reconciling task", "task", task.Name, "namespace", task.Namespace, "phase", task.Status.Phase)

	// 检查任务是否完成
	if p.isTaskCompleted(task.Status.Phase) {
		p.deleteSandboxNetworkPolicy(ctx, task)
		logger.Info("任务已完成，无需重复处理", "状态", task.Status.Phase)
		return ctrl.Result{}, nil
	}

	// 初始化任务状态为 Pending
	if task.Status.Phase == "" {
		now := metav1.Now()
		task.Status.Phase = agentflowiov1alpha1.TaskPhasePending
		task.Status.Message = "等待执行"
		task.Status.StartTime = &now
		task.Status.StartTimeUnix = now.Unix()
		if err := p.Status().Update(ctx, task); err != nil {
			return ctrl.Result{}, fmt.Errorf("更新任务状态为 Pending 失败：%w", err)
		}
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// 第一阶段：创建 Sandbox（如果还没有）
	if task.Status.Phase == agentflowiov1alpha1.TaskPhasePending {
		if skipSandboxEnv() {
			now := metav1.Now()
			task.Status.Phase = agentflowiov1alpha1.TaskPhaseRunning
			task.Status.WorkerName = localSandboxSkipName
			task.Status.Message = "本地模式：跳过 Sandbox，直接 AI 执行"
			if task.Status.StartTime == nil {
				task.Status.StartTime = &now
				task.Status.StartTimeUnix = now.Unix()
			}
			if err := p.Status().Update(ctx, task); err != nil {
				return ctrl.Result{}, fmt.Errorf("更新任务状态失败：%w", err)
			}
			p.publishTaskEvent(ctx, task, cache.EventStepSandbox, "已跳过 Sandbox（AGENTFLOW_SKIP_SANDBOX）", 0, 0)
			logger.Info("跳过 Sandbox，进入 AI 执行", "Task", task.Name)
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}

		sandboxName := fmt.Sprintf("%s-sandbox", task.Name)
		existingSandbox := &sandboxv1beta1.Sandbox{}
		err := p.Get(ctx, types.NamespacedName{Name: sandboxName, Namespace: task.Namespace}, existingSandbox)
		if k8serrors.IsNotFound(err) {
			// 创建 Sandbox
			sandbox, err := p.createSandbox(ctx, task)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("创建 Sandbox 失败：%w", err)
			}
			now := metav1.Now()
			task.Status.Phase = agentflowiov1alpha1.TaskPhaseRunning
			task.Status.Message = fmt.Sprintf("Sandbox 已创建：%s", sandbox.Name)
			task.Status.WorkerName = sandbox.Name
			if task.Status.StartTime == nil {
				task.Status.StartTime = &now
				task.Status.StartTimeUnix = now.Unix()
			}
			if err := p.Status().Update(ctx, task); err != nil {
				return ctrl.Result{}, fmt.Errorf("更新任务状态失败：%w", err)
			}
			if err := p.ensureSandboxNetworkPolicy(ctx, task); err != nil {
				return ctrl.Result{}, fmt.Errorf("创建 Sandbox 网络策略失败：%w", err)
			}
			p.publishTaskEvent(ctx, task, cache.EventStepSandbox, fmt.Sprintf("Sandbox 已创建: %s", sandbox.Name), 0, 0)
			logger.Info("任务状态已更新为 Running", "Task", task.Name, "Sandbox", sandbox.Name)
			return ctrl.Result{RequeueAfter: time.Second}, nil
		}
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("获取 Sandbox 失败：%w", err)
		}
		// Sandbox 已存在，继续
		return ctrl.Result{RequeueAfter: time.Second}, nil
	}

	// 第二阶段：Running — 等待 Sandbox 完成（或本地跳过）
	if task.Status.Phase == agentflowiov1alpha1.TaskPhaseRunning {
		sandboxSkipped := task.Status.WorkerName == localSandboxSkipName
		if !sandboxSkipped {
			sandboxName := task.Status.WorkerName
			sandbox := &sandboxv1beta1.Sandbox{}
			if err := p.Get(ctx, types.NamespacedName{Name: sandboxName, Namespace: task.Namespace}, sandbox); err != nil {
				if k8serrors.IsNotFound(err) {
					logger.Info("Sandbox 已不存在", "SandboxName", sandboxName)
					return ctrl.Result{}, nil
				}
				return ctrl.Result{}, fmt.Errorf("获取 Sandbox 失败：%w", err)
			}

			condition := meta.FindStatusCondition(sandbox.Status.Conditions, string(sandboxv1beta1.SandboxConditionFinished))
			if condition == nil {
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}

			now := metav1.Now()
			switch condition.Reason {
			case sandboxv1beta1.SandboxReasonPodSucceeded:
				logger.Info("Sandbox 执行完成", "SandboxName", sandboxName)
			case sandboxv1beta1.SandboxReasonPodFailed:
				task.Status.Phase = agentflowiov1alpha1.TaskPhaseFailed
				task.Status.Message = fmt.Sprintf("Sandbox 执行失败：%s", condition.Message)
				if task.Status.CompletionTime == nil {
					task.Status.CompletionTime = &now
					task.Status.CompletionTimeUnix = now.Unix()
				}
				if err := p.Status().Update(ctx, task); err != nil {
					return ctrl.Result{}, fmt.Errorf("更新 Task 状态失败：%w", err)
				}
				return ctrl.Result{}, nil
			}
		}

		p.publishTaskEvent(ctx, task, cache.EventStepSandbox, "开始 AI 处理", 0, 0)

		if p.isMCPTask(task) {
			return p.handleMCPTask(ctx, task, sandboxSkipped)
		}

		workerInstruction := p.buildWorkerInstruction(task)
		qualityThreshold := p.resolveQualityThreshold(task)

		if !p.needsQualityCheck(task) {
			workerOnly := flow.NewAgentFlow()
			workerOnly.AddNode(&flow.WorkerNode{Name: "worker"})
			if err := workerOnly.Compile(); err != nil {
				return ctrl.Result{}, fmt.Errorf("编译 Worker 流程失败：%w", err)
			}

			taskType := ""
			if task.Annotations != nil {
				taskType = task.Annotations["agentflow.io/monitor-task-type"]
			}
			policy := p.resolveOutputRetryPolicy(task, taskType, retryutil.FailureUnknown)
			maxAttempts := policy.MaxAttempts

			var workerState *flow.State
			var lastErr error
			feedback := ""
			for attempt := 0; attempt < maxAttempts; attempt++ {
				if attempt > 0 {
					delay := retryutil.Backoff(attempt, policy.BaseDelaySec, policy.MaxDelaySec)
					if task.Spec.RetryPolicy.RetryDelaySeconds > 0 && attempt == 1 {
						delay = time.Duration(task.Spec.RetryPolicy.RetryDelaySeconds) * time.Second
					}
					logger.Info("worker output retry backoff", "task", task.Name, "attempt", attempt+1, "delay", delay)
					if err := retryutil.Sleep(ctx, delay); err != nil {
						return ctrl.Result{}, err
					}
				}

				state, err := workerOnly.Execute(ctx, flow.State{
					Task:              task,
					Context:           ctx,
					Client:            p.Client,
					AIService:         p.AIService,
					WorkerInstruction: workerInstruction,
					MonitorFeedback:   feedback,
				})
				if err != nil {
					return ctrl.Result{}, fmt.Errorf("worker 执行失败：%w", err)
				}
				if err := flow.ValidateWorkerOutput(workerInstruction, state.WorkerOutput, taskType); err != nil {
					lastErr = err
					kind := retryutil.Classify(err)
					feedback = retryutil.OutputFeedback(kind, err, attempt)
					if attempt+1 < maxAttempts && retryutil.IsRetryable(kind) {
						logger.Info("worker output validation failed, will retry",
							"task", task.Name, "attempt", attempt+1, "maxAttempts", maxAttempts, "kind", kind, "error", err)
						continue
					}
					task.Status.Phase = agentflowiov1alpha1.TaskPhaseFailed
					task.Status.Message = fmt.Sprintf("产出校验失败（已重试 %d 次）：%v", attempt, lastErr)
					now := metav1.Now()
					if task.Status.CompletionTime == nil {
						task.Status.CompletionTime = &now
						task.Status.CompletionTimeUnix = now.Unix()
					}
					if err := p.Status().Update(ctx, task); err != nil {
						return ctrl.Result{}, fmt.Errorf("更新 Task 状态失败：%w", err)
					}
					return ctrl.Result{}, nil
				}
				workerState = state
				break
			}

			now := metav1.Now()
			if task.Status.Output == nil {
				task.Status.Output = &agentflowiov1alpha1.TaskOutput{}
			}
			task.Status.Output.Content = workerState.WorkerOutput
			task.Status.Output.Format = "text"
			task.Status.Output.GeneratedAt = &now
			task.Status.TokenUsage = ai.ToTaskTokenUsage(workerState.TokenUsage)
			task.Status.Phase = agentflowiov1alpha1.TaskPhaseSucceeded
			task.Status.Message = "任务执行成功（已跳过质量检查）"
			if task.Status.CompletionTime == nil {
				task.Status.CompletionTime = &now
				task.Status.CompletionTimeUnix = now.Unix()
			}
			if err := p.Status().Update(ctx, task); err != nil {
				return ctrl.Result{}, fmt.Errorf("更新 Task 状态失败：%w", err)
			}
			metrics.RecordTaskCompletion(string(task.Status.Phase))
			p.publishTaskEvent(ctx, task, cache.EventStepSucceeded, task.Status.Message, 0, 100)
			_ = p.clearCheckpoint(ctx, task)
			p.sendTaskFeedback(task, workerState.WorkerOutput)
			return ctrl.Result{}, nil
		}

		startRetry, feedback := p.loadCheckpoint(ctx, task, workerInstruction, qualityThreshold)
		policy := p.resolveOutputRetryPolicy(task, "", retryutil.FailureQuality)
		maxRetries := policy.MaxAttempts - 1
		if maxRetries < 0 {
			maxRetries = 0
		}

		result, err := p.runWorkerMonitorLoop(ctx, task, workerInstruction, qualityThreshold, startRetry, maxRetries, policy, feedback)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("Worker-Monitor 流程失败：%w", err)
		}

		// 更新 Task 最终状态
		now := metav1.Now()
		if task.Status.CompletionTime == nil {
			task.Status.CompletionTime = &now
			task.Status.CompletionTimeUnix = now.Unix()
		}
		if err := p.Status().Update(ctx, task); err != nil {
			return ctrl.Result{}, fmt.Errorf("更新 Task 最终状态失败：%w", err)
		}

		logger.Info("Task 最终状态已更新", "Task", task.Name, "Phase", task.Status.Phase, "result", result)

		switch task.Status.Phase {
		case agentflowiov1alpha1.TaskPhaseSucceeded:
			score := int32(0)
			if task.Status.QualityCheck != nil {
				score = task.Status.QualityCheck.Score
			}
			metrics.RecordTaskCompletion(string(task.Status.Phase))
			p.publishTaskEvent(ctx, task, cache.EventStepSucceeded, task.Status.Message, int(task.Status.Retries), int(score))
		case agentflowiov1alpha1.TaskPhaseFailed:
			metrics.RecordTaskCompletion(string(task.Status.Phase))
			p.publishTaskEvent(ctx, task, cache.EventStepFailed, task.Status.Message, int(task.Status.Retries), 0)
		}
		_ = p.clearCheckpoint(ctx, task)

		// 发送任务完成反馈到对话
		workerOutput := ""
		if task.Status.Output != nil {
			workerOutput = task.Status.Output.Content
		}
		p.sendTaskFeedback(task, workerOutput)

		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// runWorkerMonitorLoop 执行 Worker -> Monitor 循环（含重试）
func (p *TaskPlannerEino) runWorkerMonitorLoop(ctx context.Context, task *agentflowiov1alpha1.Task, instruction string, qualityThreshold, startRetry, maxRetries int, policy retryutil.Policy, initialFeedback string) (string, error) {
	logger := log.FromContext(ctx)
	feedback := initialFeedback
	var totalUsage ai.TokenUsage

	if startRetry > maxRetries {
		logger.Info("checkpoint retry exceeds budget, resetting", "task", task.Name, "startRetry", startRetry, "maxRetries", maxRetries)
		_ = p.clearCheckpoint(ctx, task)
		startRetry = 0
		feedback = initialFeedback
	}

	for attempt := startRetry; attempt <= maxRetries; attempt++ {
		if attempt > startRetry {
			delay := retryutil.Backoff(attempt-startRetry+1, policy.BaseDelaySec, policy.MaxDelaySec)
			logger.Info("worker-monitor retry backoff", "task", task.Name, "retry", attempt+1, "delay", delay)
			if err := retryutil.Sleep(ctx, delay); err != nil {
				return "", err
			}
		}
		monitorTaskType, consistencyCtx := p.buildMonitorContext(ctx, task)
		arcBoundary, firstChapter := p.chapterMonitorFlags(ctx, task)
		monitorTier := flow.ResolveMonitorTier(monitorTaskType, attempt, arcBoundary, firstChapter)

		logger.Info("开始 Worker-Monitor 循环", "retry", attempt, "maxRetries", maxRetries, "monitorTier", monitorTier)
		p.publishTaskEvent(ctx, task, cache.EventStepWorker, fmt.Sprintf("Worker 开始执行 (第 %d 次)", attempt+1), attempt, 0)
		p.saveCheckpoint(ctx, task, &cache.Checkpoint{
			Retry:             attempt,
			MonitorFeedback:   feedback,
			WorkerInstruction: instruction,
			QualityThreshold:  qualityThreshold,
			Phase:             cache.CheckpointPhaseExecuting,
			UpdatedAt:         cacheFormatNow(),
		})

		// 创建 eino 流程：Worker -> [Polish] -> Monitor
		// Use debug flow when AGENTFLOW_DEBUG_EINO=true for better visibility
		var f *flow.AgentFlow
		if os.Getenv("AGENTFLOW_DEBUG_EINO") == "true" {
			f = flow.NewAgentFlowWithDebug()
		} else {
			f = flow.NewAgentFlow()
		}
		f.AddNode(&flow.WorkerNode{Name: "worker"})
		if p.isChapterPolishTask(task) {
			f.AddNode(&flow.PolishNode{Name: "polish"})
		}
		f.AddNode(&flow.MonitorNode{Name: "monitor"})
		if err := f.Compile(); err != nil {
			return "", fmt.Errorf("编译 eino 流程失败：%w", err)
		}
		if os.Getenv("AGENTFLOW_DEBUG_EINO") == "true" {
			logger.Info("eino flow graph", "description", f.Describe())
		}

		// 初始化状态
		state := flow.State{
			Task:               task,
			Context:            ctx,
			Client:             p.Client,
			AIService:          p.AIService,
			RetryCount:         attempt,
			MaxRetries:         maxRetries,
			QualityThreshold:   qualityThreshold,
			WorkerInstruction:  instruction,
			MonitorFeedback:    feedback,
			MonitorTaskType:    monitorTaskType,
			ConsistencyContext: consistencyCtx,
			MonitorTier:        monitorTier,
			TeamMode:           p.isTeamModeTask(task),
		}

		// 执行流程
		output, err := f.Execute(ctx, state)
		if err != nil {
			return "", fmt.Errorf("eino 流程执行失败：%w", err)
		}

		totalUsage.Add(output.TokenUsage)
		logger.Info("Worker-Monitor 流程完成",
			"retry", attempt,
			"workerOutput长度", len(output.WorkerOutput),
			"monitorFeedback", output.MonitorFeedback,
			"executionResult", output.ExecutionResult,
			"tokenUsage", output.TokenUsage.TotalTokens,
		)

		p.publishTaskEvent(ctx, task, cache.EventStepMonitor, output.ExecutionResult, attempt, int(p.evalScore(output)))
		p.saveCheckpoint(ctx, task, &cache.Checkpoint{
			Retry:             attempt,
			WorkerOutput:      output.WorkerOutput,
			MonitorFeedback:   output.MonitorFeedback,
			WorkerInstruction: instruction,
			QualityThreshold:  qualityThreshold,
			Phase:             cache.CheckpointPhaseCompleted,
			UpdatedAt:         cacheFormatNow(),
		})

		// 更新 Task 的产出物
		now := metav1.Now()
		if task.Status.Output == nil {
			task.Status.Output = &agentflowiov1alpha1.TaskOutput{}
		}
		task.Status.Output.Content = output.WorkerOutput
		task.Status.Output.Format = "text"
		task.Status.Output.GeneratedAt = &now
		task.Status.TokenUsage = ai.ToTaskTokenUsage(totalUsage)

		// 更新重试次数
		task.Status.Retries = int32(attempt)

		passed := p.checkMonitorPassed(output)
		qualityScore := p.evalScore(output)
		p.applyEvalToTask(task, output, qualityScore, passed, &now)
		p.saveEvalHistory(ctx, task, output, attempt)

		if err := p.Status().Update(ctx, task); err != nil {
			return "", fmt.Errorf("更新 Task 状态失败：%w", err)
		}

		if passed {
			// 质量通过
			task.Status.Phase = agentflowiov1alpha1.TaskPhaseSucceeded
			task.Status.Message = fmt.Sprintf("任务执行成功，质量评分: %d", qualityScore)
			return fmt.Sprintf("质量通过: score=%d", qualityScore), nil
		}

		feedback = output.MonitorFeedback
		if output.MonitorEval != nil {
			feedback = flow.FormatRetryFeedback(output.MonitorEval, qualityThreshold)
		}
		logger.Info("质量检查未通过，准备重试", "score", qualityScore, "feedback", feedback)

		if attempt >= maxRetries {
			// 达到最大重试次数
			task.Status.Phase = agentflowiov1alpha1.TaskPhaseFailed
			task.Status.Message = fmt.Sprintf("质量检查未通过（score=%d），已达最大重试次数 %d", qualityScore, maxRetries)
			return fmt.Sprintf("质量不通过: score=%d, max retries reached", qualityScore), nil
		}
	}

	return "", fmt.Errorf("未预期的循环结束")
}

func (p *TaskPlannerEino) resolveOutputRetryPolicy(task *agentflowiov1alpha1.Task, taskType string, kind retryutil.FailureKind) retryutil.Policy {
	global := retryutil.Policy{
		MaxAttempts:  p.Retry.MaxRetries,
		BaseDelaySec: p.Retry.BaseDelaySec,
		MaxDelaySec:  p.Retry.MaxDelaySec,
	}
	if global.MaxAttempts <= 0 {
		global.MaxAttempts = 3
	}
	if global.BaseDelaySec <= 0 {
		global.BaseDelaySec = retryutil.DefaultBaseDelaySec
	}
	if global.MaxDelaySec <= 0 {
		global.MaxDelaySec = retryutil.DefaultMaxDelaySec
	}

	taskMax := task.Spec.RetryPolicy.MaxRetries
	if taskMax <= 0 {
		taskMax = int32(global.MaxAttempts)
	}

	isChapter := taskType == flow.TaskTypeNovelChapter
	if task.Annotations != nil && task.Annotations["agentflow.io/monitor-task-type"] == flow.TaskTypeNovelChapter {
		isChapter = true
	}
	if task.Labels != nil && task.Labels["agentflow.io/workflow"] != "" {
		if stepID := task.Labels["agentflow.io/workflow-step"]; stepID != "" {
			if _, ok := wfengine.ChapterNumFromStepID(stepID); ok {
				isChapter = true
				if task.Spec.RetryPolicy.MaxRetries <= 0 {
					taskMax = wfengine.DefaultTaskMaxRetries
				}
			}
		}
	}

	return retryutil.ResolvePolicy(taskMax, p.Quality.MaxRetries, global, kind, isChapter)
}

// buildWorkerInstruction 构建 Worker 执行指令
func (p *TaskPlannerEino) buildWorkerInstruction(task *agentflowiov1alpha1.Task) string {
	// 从 args 中提取任务描述（格式: echo 'Executing: 描述内容'）
	description := ""
	for _, arg := range task.Spec.Args {
		if strings.Contains(arg, "Executing:") {
			// 提取 Executing: 后面的内容
			idx := strings.Index(arg, "Executing:")
			if idx != -1 {
				desc := strings.TrimSpace(arg[idx+len("Executing:"):])
				desc = strings.Trim(desc, "'\"")
				description = desc
			}
		}
	}

	if description != "" {
		applog.Component("planner").Debug("worker instruction built", "instruction", description)
		return description
	}

	// 降级：使用 Task 名称
	return fmt.Sprintf("执行任务: %s", task.Name)
}

func (p *TaskPlannerEino) buildMonitorContext(ctx context.Context, task *agentflowiov1alpha1.Task) (string, string) {
	taskType := ""
	if task.Annotations != nil {
		taskType = task.Annotations["agentflow.io/monitor-task-type"]
	}
	if taskType == flow.TaskTypeNovelOutlineRefine {
		wfName := ""
		if task.Labels != nil {
			wfName = task.Labels["agentflow.io/workflow"]
		}
		if wfName != "" {
			wf := &agentflowiov1alpha1.Workflow{}
			if err := p.Get(ctx, types.NamespacedName{Namespace: task.Namespace, Name: wfName}, wf); err == nil {
				if wf.Status.WorkspacePath == "" {
					wf.Status.WorkspacePath = wfengine.WorkspacePath(wf)
				}
				return taskType, wfengine.BuildOutlineRefineMonitorContext(wf)
			}
		}
		return taskType, ""
	}
	if taskType != flow.TaskTypeNovelChapter && taskType != flow.TaskTypeNovelChapterTeam {
		return taskType, ""
	}

	wfName := ""
	stepID := ""
	if task.Labels != nil {
		wfName = task.Labels["agentflow.io/workflow"]
		stepID = task.Labels["agentflow.io/workflow-step"]
	}
	if wfName == "" || stepID == "" {
		return taskType, ""
	}

	wf := &agentflowiov1alpha1.Workflow{}
	if err := p.Get(ctx, types.NamespacedName{Namespace: task.Namespace, Name: wfName}, wf); err != nil {
		return taskType, ""
	}
	if wf.Status.WorkspacePath == "" {
		wf.Status.WorkspacePath = wfengine.WorkspacePath(wf)
	}
	return taskType, wfengine.BuildConsistencyMonitorContext(wf, stepID)
}

func (p *TaskPlannerEino) resolveQualityThreshold(task *agentflowiov1alpha1.Task) int {
	if ann := task.Annotations["agentflow.io/quality-threshold"]; ann != "" {
		if v, err := strconv.Atoi(ann); err == nil && v > 0 && v <= 100 {
			return v
		}
	}
	if p.AIService != nil && p.AIService.Config() != nil && p.AIService.Config().Quality.Threshold > 0 {
		return p.AIService.Config().Quality.Threshold
	}
	return wfengine.DefaultQualityThreshold
}

func (p *TaskPlannerEino) needsQualityCheck(task *agentflowiov1alpha1.Task) bool {
	if task.Labels != nil {
		if v, ok := task.Labels["agentflow.io/needs-quality-check"]; ok && strings.EqualFold(v, "false") {
			return false
		}
	}
	// 工作流章节：分层质检（L0 规则 + L1 轻量 AI / L2 全量），即使跳过 Sandbox 也启用。
	return true
}

// chapterMonitorFlags reports arc-boundary and first-chapter for tier escalation.
func (p *TaskPlannerEino) isTeamModeTask(task *agentflowiov1alpha1.Task) bool {
	if task == nil || task.Annotations == nil {
		return false
	}
	if strings.EqualFold(task.Annotations["agentflow.io/team-mode"], "true") {
		return true
	}
	return task.Annotations["agentflow.io/monitor-task-type"] == flow.TaskTypeNovelChapterTeam
}

// isChapterPolishTask limits line-editing polish to chapter prose, not JSON steps (outline/bible).
func (p *TaskPlannerEino) isChapterPolishTask(task *agentflowiov1alpha1.Task) bool {
	if task == nil || task.Annotations == nil {
		return false
	}
	return task.Annotations["agentflow.io/monitor-task-type"] == flow.TaskTypeNovelChapterTeam
}

func (p *TaskPlannerEino) chapterMonitorFlags(ctx context.Context, task *agentflowiov1alpha1.Task) (arcBoundary, firstChapter bool) {
	if task.Labels == nil {
		return false, false
	}
	wfName := task.Labels["agentflow.io/workflow"]
	stepID := task.Labels["agentflow.io/workflow-step"]
	if wfName == "" || stepID == "" {
		return false, false
	}
	num, ok := wfengine.ChapterNumFromStepID(stepID)
	if !ok || num <= 0 {
		return false, false
	}
	if num == 1 {
		return false, true
	}
	wf := &agentflowiov1alpha1.Workflow{}
	if err := p.Get(ctx, types.NamespacedName{Namespace: task.Namespace, Name: wfName}, wf); err != nil {
		return false, false
	}
	interval := wfengine.DefaultArcInterval(wf.Spec.Params, wfengine.OutlineChapterCount(wf))
	if interval <= 0 {
		interval = 10
	}
	return num%interval == 0, false
}

func (p *TaskPlannerEino) checkMonitorPassed(output *flow.State) bool {
	if output.MonitorEval != nil {
		return output.MonitorEval.Passed
	}
	return p.evalScore(output) >= int32(output.QualityThreshold)
}

func (p *TaskPlannerEino) evalScore(output *flow.State) int32 {
	if output.MonitorEval != nil {
		return int32(output.MonitorEval.Score)
	}
	return 0
}

func (p *TaskPlannerEino) applyEvalToTask(task *agentflowiov1alpha1.Task, output *flow.State, score int32, passed bool, evaluatedAt *metav1.Time) {
	if task.Status.QualityCheck == nil {
		task.Status.QualityCheck = &agentflowiov1alpha1.QualityCheck{}
	}
	qc := task.Status.QualityCheck
	qc.Score = score
	qc.Passed = passed
	qc.EvaluatedAt = evaluatedAt
	qc.Attempt = int32(output.RetryCount)

	if output.MonitorEval != nil {
		eval := output.MonitorEval
		qc.Feedback = eval.Feedback
		qc.TaskType = eval.TaskType
		qc.CheckMethod = eval.CheckMethod
		qc.Issues = append([]string(nil), eval.Issues...)
		if eval.Dimensions != nil {
			qc.Dimensions = &agentflowiov1alpha1.QualityDimensions{
				Completeness: int32(eval.Dimensions.Completeness),
				Accuracy:     int32(eval.Dimensions.Accuracy),
				Quality:      int32(eval.Dimensions.Quality),
			}
		}
		if !passed {
			qc.Feedback = flow.FormatRetryFeedback(eval, output.QualityThreshold)
		}
	} else {
		qc.Feedback = output.MonitorFeedback
	}
}

func (p *TaskPlannerEino) saveEvalHistory(ctx context.Context, task *agentflowiov1alpha1.Task, output *flow.State, retry int) {
	if p.Store == nil || output.MonitorEval == nil {
		return
	}
	eval := output.MonitorEval
	_ = p.Store.AppendEvalHistory(ctx, task.Namespace, task.Name, cache.EvalRecord{
		Attempt:     retry,
		Score:       eval.Score,
		Passed:      eval.Passed,
		Feedback:    eval.Feedback,
		Issues:      append([]string(nil), eval.Issues...),
		TaskType:    eval.TaskType,
		CheckMethod: eval.CheckMethod,
		Timestamp:   cacheFormatNow(),
	})
}

// createSandbox 创建官方 agent-sandbox，执行真实 AI 任务
// 检查 annotation agentflow.io/mcp-mode=true 时创建带 MCP sidecar 的增强沙箱
func (p *TaskPlannerEino) createSandbox(ctx context.Context, task *agentflowiov1alpha1.Task) (*sandboxv1beta1.Sandbox, error) {
	if task.Annotations["agentflow.io/mcp-mode"] == "true" {
		return p.createEnhancedSandbox(ctx, task)
	}
	return p.createBasicSandbox(ctx, task)
}

// createBasicSandbox 创建基础沙箱（仅执行脚本，AI 在控制器中完成）
func (p *TaskPlannerEino) createBasicSandbox(ctx context.Context, task *agentflowiov1alpha1.Task) (*sandboxv1beta1.Sandbox, error) {
	sandboxName := fmt.Sprintf("%s-sandbox", task.Name)

	description := p.extractTaskDescription(task)

	execScript := fmt.Sprintf(`#!/bin/sh
TASK_DESC="%s"
TASK_NAME="%s"

echo "任务执行中: ${TASK_DESC}"

mkdir -p /data/outputs/default
echo "${TASK_DESC}" > /data/outputs/default/${TASK_NAME}-instruction.txt

echo "任务执行完成"
`, description, task.Name)

	agentSandbox := &sandboxv1beta1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sandboxName,
			Namespace: task.Namespace,
			Labels: map[string]string{
				"agents.x-k8s.io/sandbox":   sandboxName,
				"agentflow.io/task":         task.Name,
				"agentflow.io/sandbox-kind": "agent-sandbox",
				"agentflow.io/mcp-mode":     "false",
			},
			Annotations: map[string]string{
				"agentflow.io/task-uid": string(task.UID),
			},
		},
		Spec: sandboxv1beta1.SandboxSpec{
			PodTemplate: sandboxv1beta1.PodTemplate{
				Spec: func() corev1.PodSpec {
					spec := corev1.PodSpec{
						RestartPolicy:    corev1.RestartPolicyNever,
						RuntimeClassName: task.Spec.RuntimeClassName,
						SecurityContext:  task.Spec.PodSecurityContext,
						Containers: []corev1.Container{
							{
								Name:            task.Name,
								Image:           task.Spec.Image,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         []string{"/bin/sh", "-c"},
								Args:            []string{execScript},
								Env: []corev1.EnvVar{
									{Name: "TASK_NAME", Value: task.Name},
									{Name: "TASK_DESC", Value: description},
								},
								Resources: corev1.ResourceRequirements{
									Limits: task.Spec.Resources.Limits,
								},
								VolumeMounts: []corev1.VolumeMount{
									{Name: "task-outputs", MountPath: "/data/outputs"},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "task-outputs",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "task-outputs",
									},
								},
							},
						},
					}
					applySandboxPodDefaults(&spec)
					return spec
				}(),
				ObjectMeta: sandboxv1beta1.PodMetadata{
					Labels: sandboxPodLabels(task, sandboxName),
				},
			},
			Lifecycle: sandboxv1beta1.Lifecycle{
				ShutdownTime:   nil,
				ShutdownPolicy: nil,
			},
			OperatingMode: sandboxv1beta1.SandboxOperatingModeRunning,
		},
	}

	if err := p.Create(ctx, agentSandbox); err != nil {
		return nil, err
	}
	return agentSandbox, nil
}

// createEnhancedSandbox 创建增强沙箱（MCP sidecar + Worker Agent）
// Sandbox Pod 包含两个容器：
//   - worker-agent: 运行 AI Agent 循环，调用 MCP sidecar 获取工具
//   - mcp-sidecar: 提供工具执行（shell/file/http）和 AI 代理
func (p *TaskPlannerEino) createEnhancedSandbox(ctx context.Context, task *agentflowiov1alpha1.Task) (*sandboxv1beta1.Sandbox, error) {
	sandboxName := fmt.Sprintf("%s-sandbox", task.Name)
	description := p.extractTaskDescription(task)

	mcpImage := os.Getenv("MCP_IMAGE")
	if mcpImage == "" {
		mcpImage = "minagflow/mcp-sidecar:latest"
	}
	workerImage := os.Getenv("WORKER_AGENT_IMAGE")
	if workerImage == "" {
		workerImage = "minagflow/worker-agent:latest"
	}

	agentSandbox := &sandboxv1beta1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sandboxName,
			Namespace: task.Namespace,
			Labels: map[string]string{
				"agents.x-k8s.io/sandbox":   sandboxName,
				"agentflow.io/task":         task.Name,
				"agentflow.io/sandbox-kind": "agent-sandbox",
				"agentflow.io/mcp-mode":     "true",
			},
			Annotations: map[string]string{
				"agentflow.io/task-uid": string(task.UID),
			},
		},
		Spec: sandboxv1beta1.SandboxSpec{
			PodTemplate: sandboxv1beta1.PodTemplate{
				Spec: func() corev1.PodSpec {
					spec := corev1.PodSpec{
						ServiceAccountName: "sandbox-reader",
						RestartPolicy:      corev1.RestartPolicyNever,
						RuntimeClassName:   task.Spec.RuntimeClassName,
						SecurityContext:    task.Spec.PodSecurityContext,
						Containers: []corev1.Container{
							{
								Name:            "mcp-sidecar",
								Image:           mcpImage,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         []string{"/usr/local/bin/mcp-server"},
								Args: []string{
									"--port=9090",
									"--ai-base-url=" + os.Getenv("AI_BASE_URL"),
									"--ai-api-key=" + os.Getenv("AI_API_KEY"),
									"--ai-model=" + getEnvDefault("AI_MODEL", "Qwen3.5-35B-A3B"),
								},
								Ports: []corev1.ContainerPort{
									{Name: "mcp", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
								},
								Env: []corev1.EnvVar{
									{Name: "AI_BASE_URL", Value: os.Getenv("AI_BASE_URL")},
									{Name: "AI_API_KEY", Value: os.Getenv("AI_API_KEY")},
									{Name: "AI_MODEL", Value: getEnvDefault("AI_MODEL", "Qwen3.5-35B-A3B")},
								},
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										"cpu":    resource.MustParse("500m"),
										"memory": resource.MustParse("256Mi"),
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{Name: "task-outputs", MountPath: "/data/outputs"},
								},
							},
							{
								Name:            "worker-agent",
								Image:           workerImage,
								ImagePullPolicy: corev1.PullIfNotPresent,
								Command:         []string{"/usr/local/bin/worker-agent"},
								Args: []string{
									"--mcp-url=http://localhost:9090",
									"--task=" + description,
									"--task-name=" + task.Name,
									"--output-dir=/data/outputs",
									"--max-steps=15",
								},
								Env: []corev1.EnvVar{
									{Name: "MCP_URL", Value: "http://localhost:9090"},
									{Name: "TASK_DESC", Value: description},
									{Name: "TASK_NAME", Value: task.Name},
									{Name: "OUTPUT_DIR", Value: "/data/outputs"},
								},
								Resources: corev1.ResourceRequirements{
									Limits: task.Spec.Resources.Limits,
								},
								VolumeMounts: []corev1.VolumeMount{
									{Name: "task-outputs", MountPath: "/data/outputs"},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "task-outputs",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: "task-outputs",
									},
								},
							},
						},
					}
					applySandboxPodDefaults(&spec)
					return spec
				}(),
				ObjectMeta: sandboxv1beta1.PodMetadata{
					Labels: sandboxPodLabels(task, sandboxName),
				},
			},
			Lifecycle: sandboxv1beta1.Lifecycle{
				ShutdownTime:   nil,
				ShutdownPolicy: nil,
			},
			OperatingMode: sandboxv1beta1.SandboxOperatingModeRunning,
		},
	}

	if err := p.Create(ctx, agentSandbox); err != nil {
		return nil, err
	}
	return agentSandbox, nil
}

// extractTaskDescription 从 Task args 中提取任务描述
func (p *TaskPlannerEino) extractTaskDescription(task *agentflowiov1alpha1.Task) string {
	for _, arg := range task.Spec.Args {
		if strings.Contains(arg, "Executing:") {
			idx := strings.Index(arg, "Executing:")
			if idx != -1 {
				desc := strings.TrimSpace(arg[idx+len("Executing:"):])
				desc = strings.Trim(desc, "'\"")
				return desc
			}
		}
	}
	return task.Name
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// isTaskCompleted 检查任务是否完成
func (p *TaskPlannerEino) isTaskCompleted(phase agentflowiov1alpha1.TaskPhase) bool {
	switch phase {
	case agentflowiov1alpha1.TaskPhaseSucceeded, agentflowiov1alpha1.TaskPhaseFailed:
		return true
	default:
		return false
	}
}

// sendTaskFeedback 发送任务完成反馈到对话
func (p *TaskPlannerEino) sendTaskFeedback(task *agentflowiov1alpha1.Task, workerOutput string) {
	// 工作流子任务由 WorkflowController 汇总进度，避免每章刷屏
	if task.Labels != nil && task.Labels["agentflow.io/workflow"] != "" {
		return
	}

	// 构建反馈消息
	var feedback string
	if task.Status.Phase == agentflowiov1alpha1.TaskPhaseSucceeded {
		score := int32(0)
		if task.Status.QualityCheck != nil {
			score = task.Status.QualityCheck.Score
		}
		feedback = fmt.Sprintf("✅ 任务「%s」已完成！\n\n评分: %d/100\n产出物摘要: %s",
			task.Name, score, truncateString(workerOutput, 200))
	} else {
		feedback = fmt.Sprintf("❌ 任务「%s」执行失败\n\n原因: %s", task.Name, task.Status.Message)
	}

	// 直接写入 ChatRouter 的对话历史
	feedbackLog := applog.Component("feedback")
	if p.ChatRouter != nil {
		p.ChatRouter.AddSystemMessage(feedback)
		feedbackLog.Info("task feedback written to conversation", "task", task.Name)
	} else {
		feedbackLog.Warn("chat router not initialized, skipping feedback", "task", task.Name)
	}
}

// truncateString 截断字符串
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// SetupWithManager 设置控制器管理器
func (p *TaskPlannerEino) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentflowiov1alpha1.Task{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: taskConcurrentReconciles()}).
		Complete(p)
}

func taskConcurrentReconciles() int {
	if raw := strings.TrimSpace(os.Getenv("AGENTFLOW_MAX_CONCURRENT_TASKS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultTaskConcurrentReconciles
}

func (p *TaskPlannerEino) loadCheckpoint(ctx context.Context, task *agentflowiov1alpha1.Task, instruction string, qualityThreshold int) (int, string) {
	if p.Store == nil {
		return int(task.Status.Retries), ""
	}
	cp, err := p.Store.GetCheckpoint(ctx, task.Namespace, task.Name)
	if err != nil || cp == nil {
		return int(task.Status.Retries), ""
	}
	if cp.WorkerInstruction != instruction || cp.QualityThreshold != qualityThreshold {
		_ = p.Store.DeleteCheckpoint(ctx, task.Namespace, task.Name)
		return int(task.Status.Retries), ""
	}
	switch cp.Phase {
	case cache.CheckpointPhaseCompleted:
		return cp.Retry + 1, cp.MonitorFeedback
	default:
		return cp.Retry, cp.MonitorFeedback
	}
}

func (p *TaskPlannerEino) saveCheckpoint(ctx context.Context, task *agentflowiov1alpha1.Task, cp *cache.Checkpoint) {
	if p.Store == nil || cp == nil {
		return
	}
	_ = p.Store.SaveCheckpoint(ctx, task.Namespace, task.Name, cp)
}

func (p *TaskPlannerEino) clearCheckpoint(ctx context.Context, task *agentflowiov1alpha1.Task) error {
	if p.Store == nil {
		return nil
	}
	return p.Store.DeleteCheckpoint(ctx, task.Namespace, task.Name)
}

func (p *TaskPlannerEino) publishTaskEvent(ctx context.Context, task *agentflowiov1alpha1.Task, step, message string, retry, score int) {
	if p.Store == nil {
		return
	}
	_ = p.Store.PublishEvent(ctx, task.Namespace, task.Name, &cache.TaskEvent{
		TaskName:  task.Name,
		Namespace: task.Namespace,
		Step:      step,
		Message:   message,
		Retry:     retry,
		Score:     score,
		Timestamp: cacheFormatNow(),
	})
}

func cacheFormatNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

const localSandboxSkipName = "local-skip-sandbox"

func skipSandboxEnv() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("AGENTFLOW_SKIP_SANDBOX")))
	return v == "1" || v == "true" || v == "yes"
}
