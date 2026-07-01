package architecture

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/flow"
	"agent-flow/internal/store"
	wfengine "agent-flow/internal/workflow"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// WorkflowController orchestrates multi-step workflows.
type WorkflowController struct {
	client.Client
	Scheme   *runtime.Scheme
	Store    store.Store
	Notifier flow.WorkflowNotifier

	notifyMu     sync.Mutex
	lastNotified map[string]int32
}

// +kubebuilder:rbac:groups=agentflow.io,resources=workflows,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agentflow.io,resources=workflows/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agentflow.io,resources=tasks,verbs=get;list;watch;create;update;patch;delete

func (c *WorkflowController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	wf := &agentflowiov1alpha1.Workflow{}
	if err := c.Get(ctx, req.NamespacedName, wf); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if wf.Status.Phase == "" {
		if err := c.initWorkflow(ctx, wf); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if wf.Status.Phase == agentflowiov1alpha1.WorkflowPhaseSucceeded ||
		wf.Status.Phase == agentflowiov1alpha1.WorkflowPhaseFailed {
		return ctrl.Result{}, nil
	}

	if err := wfengine.EnsureWorkspace(wf); err != nil {
		return c.failWorkflow(ctx, wf, fmt.Errorf("workspace init failed: %w", err))
	}
	c.backfillStoreFromWorkspace(ctx, wf)

	if err := c.syncRunningTasks(ctx, wf); err != nil {
		return ctrl.Result{}, err
	}

	resolved, err := wfengine.ResolveSteps(wf)
	if err != nil {
		if wfengine.OutlineReady(wf) {
			return c.failWorkflow(ctx, wf, err)
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}

	wf.Status.Progress.Total = wfengine.TotalSteps(wf, resolved)
	wf.Status.Progress.Completed = c.countResolvedProgress(wf.Status.CompletedSteps, resolved)
	if wf.Status.Progress.Total > 0 {
		wf.Status.Progress.Percent = wf.Status.Progress.Completed * 100 / wf.Status.Progress.Total
	}
	c.maybeNotifyProgress(wf)

	return c.reconcileReadySteps(ctx, wf, resolved)
}

func (c *WorkflowController) reconcileReadySteps(ctx context.Context, wf *agentflowiov1alpha1.Workflow, resolved []wfengine.ResolvedStep) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var ready []wfengine.ResolvedStep
	var err error
	if wfengine.IsParallelMode(wf) {
		ready, err = wfengine.ReadySteps(wf, resolved)
	} else {
		var next *wfengine.ResolvedStep
		next, err = wfengine.NextStep(wf, resolved)
		if next != nil {
			ready = []wfengine.ResolvedStep{*next}
		}
	}
	if err != nil {
		wf.Status.Phase = agentflowiov1alpha1.WorkflowPhaseRunning
		wf.Status.Message = err.Error()
		if wf.Spec.Execution.PauseOnStepFail {
			wf.Status.Phase = agentflowiov1alpha1.WorkflowPhasePaused
		}
		if uerr := c.Status().Update(ctx, wf); uerr != nil {
			return ctrl.Result{}, uerr
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if len(ready) == 0 {
		if int(wf.Status.Progress.Completed) >= len(resolved) {
			return c.completeWorkflow(ctx, wf)
		}
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	wf.Status.Phase = agentflowiov1alpha1.WorkflowPhaseRunning
	wf.Status.CurrentStep = ready[0].ID
	if missing := c.missingChapters(ctx, wf); len(missing) > 0 {
		wf.Status.Message = fmt.Sprintf("backfilling %d missing chapters (next: chapter-%d)", len(missing), missing[0])
	}

	slots := 1
	if wfengine.IsParallelMode(wf) {
		active := c.countActiveWorkflowTasks(wf)
		slots = wfengine.MaxParallel(wf) - active
		if slots <= 0 {
			return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
		}
	}

	statusChanged := false
	var retryWait time.Duration
	for _, step := range ready {
		if slots <= 0 {
			break
		}
		if !wfengine.StepReadyForRetry(wf, step.ID) {
			if wait := wfengine.StepRetryWaitRemaining(wf, step.ID); wait > retryWait {
				retryWait = wait
			}
			continue
		}
		if step.Type == agentflowiov1alpha1.WorkflowStepTypeMerge {
			if err := c.runLocalMergeStep(ctx, wf, step); err != nil {
				return c.failWorkflow(ctx, wf, err)
			}
			return ctrl.Result{Requeue: true}, nil
		}

		taskName := taskNameForWorkflowStep(wf.Name, step.ID)
		existing := &agentflowiov1alpha1.Task{}
		err = c.Get(ctx, types.NamespacedName{Name: taskName, Namespace: wf.Namespace}, existing)
		if k8serrors.IsNotFound(err) {
			task := buildWorkflowTask(wf, step)
			if err := c.Create(ctx, task); err != nil {
				return ctrl.Result{}, err
			}
			c.upsertStepStatus(wf, step.ID, agentflowiov1alpha1.TaskPhasePending, task.Name, "task created")
			c.markChapterWriting(ctx, wf, step.ID)
			logger.Info("workflow task created", "workflow", wf.Name, "step", step.ID, "task", task.Name)
			statusChanged = true
			slots--
			if !wfengine.IsParallelMode(wf) {
				if err := c.Status().Update(ctx, wf); err != nil {
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}
			continue
		}
		if err != nil {
			return ctrl.Result{}, err
		}

		if existing.Status.Phase == agentflowiov1alpha1.TaskPhaseFailed &&
			!containsString(wf.Status.CompletedSteps, step.ID) {
			if c.tryScheduleStepRetry(ctx, wf, step.ID, existing.Name, existing.Status.Message, existing) {
				statusChanged = true
				slots--
				continue
			}
			missing := c.missingChapters(ctx, wf)
			if !wfengine.FailedStepBlocksProgressForStep(wf, step.ID, missing) {
				if err := c.Delete(ctx, existing); err != nil && !k8serrors.IsNotFound(err) {
					return ctrl.Result{}, err
				}
				wf.Status.FailedSteps = removeString(wf.Status.FailedSteps, step.ID)
				wfengine.ClearStepDispatchState(wf, step.ID)
				task := buildWorkflowTask(wf, step)
				if err := c.Create(ctx, task); err != nil {
					return ctrl.Result{}, err
				}
				c.upsertStepStatus(wf, step.ID, agentflowiov1alpha1.TaskPhasePending, task.Name, "task re-created")
				logger.Info("workflow task re-created after non-blocking failure",
					"workflow", wf.Name, "step", step.ID, "task", task.Name)
				statusChanged = true
				slots--
				continue
			}
		}

		c.upsertStepStatus(wf, step.ID, existing.Status.Phase, existing.Name, existing.Status.Message)
		statusChanged = true
		if !wfengine.IsParallelMode(wf) {
			break
		}
	}

	if statusChanged {
		if err := c.Status().Update(ctx, wf); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	if retryWait > 0 {
		log.FromContext(ctx).V(1).Info("waiting before workflow step retry", "workflow", wf.Name, "wait", retryWait)
		return ctrl.Result{RequeueAfter: retryWait}, nil
	}
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (c *WorkflowController) countActiveWorkflowTasks(wf *agentflowiov1alpha1.Workflow) int {
	active := 0
	for _, st := range wf.Status.StepStatuses {
		switch st.Phase {
		case agentflowiov1alpha1.TaskPhaseRunning, agentflowiov1alpha1.TaskPhasePending:
			active++
		}
	}
	return active
}

func (c *WorkflowController) initWorkflow(ctx context.Context, wf *agentflowiov1alpha1.Workflow) error {
	now := metav1.Now()
	wf.Status.Phase = agentflowiov1alpha1.WorkflowPhaseRunning
	wf.Status.Message = "workflow started"
	wf.Status.StartTime = &now
	wf.Status.WorkspacePath = wfengine.WorkspacePath(wf)
	wf.Status.CompletedSteps = []string{}
	wf.Status.FailedSteps = []string{}
	c.ensureNovelInStore(ctx, wf)
	return c.Status().Update(ctx, wf)
}

func (c *WorkflowController) syncRunningTasks(ctx context.Context, wf *agentflowiov1alpha1.Workflow) error {
	taskList := &agentflowiov1alpha1.TaskList{}
	if err := c.List(ctx, taskList, client.InNamespace(wf.Namespace), client.MatchingLabels{
		"agentflow.io/workflow": wf.Name,
	}); err != nil {
		return err
	}

	livePhases := make(map[string]agentflowiov1alpha1.TaskPhase, len(taskList.Items))
	for _, task := range taskList.Items {
		if stepID := task.Labels["agentflow.io/workflow-step"]; stepID != "" {
			livePhases[stepID] = task.Status.Phase
		}
	}

	changed := wfengine.ReconcileStaleStepStatuses(wf, livePhases)

	for _, task := range taskList.Items {
		stepID := task.Labels["agentflow.io/workflow-step"]
		if stepID == "" {
			continue
		}
		switch task.Status.Phase {
		case agentflowiov1alpha1.TaskPhaseSucceeded:
			if containsString(wf.Status.CompletedSteps, stepID) {
				continue
			}
			output := ""
			if task.Status.Output != nil {
				output = task.Status.Output.Content
			}
			relPath := task.Annotations["agentflow.io/workflow-output"]
			var summaryPath string
			if relPath != "" && output != "" {
				if stepID == "outline" || strings.HasSuffix(relPath, "outline.json") {
					output = flow.NormalizeWorkerOutput(`小说大纲 JSON "chapters"`, output)
					c.syncOutlineToStore(ctx, wf, output)
				}
				_ = wfengine.WriteArtifact(wf, relPath, output)
				if num, ok := wfengine.ChapterNumFromStepID(stepID); ok {
					width := wfengine.ChapterPaddingWidth(wfengine.OutlineChapterCount(wf))
					summaryPath = fmt.Sprintf("chapters/%s", wfengine.ChapterSummaryFileName(num, width))
					_ = wfengine.WriteArtifact(wf, summaryPath, wfengine.SummarizeChapter(output, 300))
				}
			}
			c.markChapterDone(ctx, wf, stepID, relPath, summaryPath, output, task.Status.QualityCheck)
			wf.Status.CompletedSteps = append(wf.Status.CompletedSteps, stepID)
			c.markStepStatus(wf, stepID, agentflowiov1alpha1.TaskPhaseSucceeded, task.Name, "completed", relPath, task.Status.QualityCheck)
			c.maybeCompleteChaptersGroup(wf)
			changed = true
		case agentflowiov1alpha1.TaskPhaseFailed:
			if c.tryScheduleStepRetry(ctx, wf, stepID, task.Name, task.Status.Message, &task) {
				changed = true
				continue
			}
			if !containsString(wf.Status.FailedSteps, stepID) {
				wf.Status.FailedSteps = append(wf.Status.FailedSteps, stepID)
				c.markChapterFailed(ctx, wf, stepID)
				c.markStepStatus(wf, stepID, agentflowiov1alpha1.TaskPhaseFailed, task.Name, task.Status.Message, "", task.Status.QualityCheck)
				changed = true
			}
		default:
			c.upsertStepStatus(wf, stepID, task.Status.Phase, task.Name, task.Status.Message)
			changed = true
		}
	}
	if changed {
		return c.Status().Update(ctx, wf)
	}
	return nil
}

func (c *WorkflowController) runLocalMergeStep(ctx context.Context, wf *agentflowiov1alpha1.Workflow, step wfengine.ResolvedStep) error {
	if containsString(wf.Status.CompletedSteps, step.ID) {
		return nil
	}

	var content string
	var err error
	message := "merged locally"

	switch step.ID {
	case "outline-merge":
		content, err = wfengine.MergeVolumeOutlines(wf)
		message = "outline merged locally"
	default:
		content, err = wfengine.MergeChapterFiles(wf)
	}

	if err != nil {
		return err
	}

	path := step.OutputPath
	if path == "" {
		if step.ID == "outline-merge" {
			path = "outline.json"
		} else {
			path = "book.md"
		}
	}
	if err := wfengine.WriteArtifact(wf, path, content); err != nil {
		return err
	}
	if step.ID == "outline-merge" {
		c.syncOutlineToStore(ctx, wf, content)
		_ = wfengine.SnapshotOutlineDraft(wf)
	}
	wf.Status.CompletedSteps = append(wf.Status.CompletedSteps, step.ID)
	c.markStepStatus(wf, step.ID, agentflowiov1alpha1.TaskPhaseSucceeded, "", message, path, nil)
	return c.Status().Update(ctx, wf)
}

func (c *WorkflowController) completeWorkflow(ctx context.Context, wf *agentflowiov1alpha1.Workflow) (ctrl.Result, error) {
	now := metav1.Now()
	wf.Status.Phase = agentflowiov1alpha1.WorkflowPhaseSucceeded
	wf.Status.Message = "workflow completed"
	wf.Status.CompletionTime = &now
	wf.Status.CurrentStep = ""
	wf.Status.Progress.Percent = 100
	if err := c.Status().Update(ctx, wf); err != nil {
		return ctrl.Result{}, err
	}
	c.notifyTerminal(wf, agentflowiov1alpha1.WorkflowPhaseSucceeded, "全部步骤已完成，可在「工作流」或「小说阅读」页查看产出。")
	return ctrl.Result{}, nil
}

func (c *WorkflowController) failWorkflow(ctx context.Context, wf *agentflowiov1alpha1.Workflow, err error) (ctrl.Result, error) {
	now := metav1.Now()
	wf.Status.Phase = agentflowiov1alpha1.WorkflowPhaseFailed
	wf.Status.Message = err.Error()
	wf.Status.CompletionTime = &now
	_ = c.Status().Update(ctx, wf)
	c.notifyTerminal(wf, agentflowiov1alpha1.WorkflowPhaseFailed, err.Error())
	return ctrl.Result{}, err
}

func (c *WorkflowController) isChatSourced(wf *agentflowiov1alpha1.Workflow) bool {
	return wf != nil && wf.Labels["agentflow.io/source"] == "chat"
}

func (c *WorkflowController) maybeNotifyProgress(wf *agentflowiov1alpha1.Workflow) {
	if c.Notifier == nil || !c.isChatSourced(wf) {
		return
	}
	pct := wf.Status.Progress.Percent
	key := wf.Namespace + "/" + wf.Name

	c.notifyMu.Lock()
	defer c.notifyMu.Unlock()
	if c.lastNotified == nil {
		c.lastNotified = make(map[string]int32)
	}
	last := c.lastNotified[key]
	milestone := (pct / 10) * 10
	if milestone <= last {
		return
	}
	c.lastNotified[key] = milestone

	msg := wf.Status.Message
	if msg == "" && wf.Status.Progress.Total > 0 {
		msg = fmt.Sprintf("已完成 %d/%d 步", wf.Status.Progress.Completed, wf.Status.Progress.Total)
	}
	c.Notifier.NotifyWorkflowEvent(flow.WorkflowEvent{
		Namespace: wf.Namespace,
		Name:      wf.Name,
		Phase:     string(wf.Status.Phase),
		Percent:   milestone,
		Message:   msg,
	})
}

func (c *WorkflowController) notifyTerminal(wf *agentflowiov1alpha1.Workflow, phase agentflowiov1alpha1.WorkflowPhase, message string) {
	if c.Notifier == nil || !c.isChatSourced(wf) {
		return
	}
	key := wf.Namespace + "/" + wf.Name
	c.notifyMu.Lock()
	c.lastNotified[key] = 100
	c.notifyMu.Unlock()

	percent := int32(0)
	if phase == agentflowiov1alpha1.WorkflowPhaseSucceeded {
		percent = 100
	}
	c.Notifier.NotifyWorkflowEvent(flow.WorkflowEvent{
		Namespace: wf.Namespace,
		Name:      wf.Name,
		Phase:     string(phase),
		Percent:   percent,
		Message:   message,
	})
}

func (c *WorkflowController) maybeCompleteChaptersGroup(wf *agentflowiov1alpha1.Workflow) {
	expected := wfengine.OutlineChapterCount(wf)
	done := 0
	for _, id := range wf.Status.CompletedSteps {
		if _, ok := wfengine.ChapterNumFromStepID(id); ok {
			done++
		}
	}
	if done >= expected && !containsString(wf.Status.CompletedSteps, "chapters") {
		wf.Status.CompletedSteps = append(wf.Status.CompletedSteps, "chapters")
	}
}

func (c *WorkflowController) countResolvedProgress(completed []string, resolved []wfengine.ResolvedStep) int32 {
	if len(resolved) == 0 {
		return int32(len(completed))
	}
	done := int32(0)
	for _, step := range resolved {
		if containsString(completed, step.ID) {
			done++
		}
	}
	return done
}

func (c *WorkflowController) tryScheduleStepRetry(ctx context.Context, wf *agentflowiov1alpha1.Workflow, stepID, taskName, message string, task *agentflowiov1alpha1.Task) bool {
	if !wfengine.StepAutoRetryEnabled(wf) {
		return false
	}
	retries := wfengine.StepStatusRetries(wf, stepID) + 1
	maxRetries := wfengine.StepMaxRetries(wf)
	if int(retries) > maxRetries {
		return false
	}

	if err := c.Delete(ctx, task); err != nil && !k8serrors.IsNotFound(err) {
		return false
	}
	flow.ClearTaskPVCOutput(task.Namespace, task.Name)

	wf.Status.FailedSteps = removeString(wf.Status.FailedSteps, stepID)
	c.markStepRetry(wf, stepID, retries, fmt.Sprintf("auto-retry %d/%d: %s", retries, maxRetries, message))
	if wf.Status.Phase == agentflowiov1alpha1.WorkflowPhasePaused {
		wf.Status.Phase = agentflowiov1alpha1.WorkflowPhaseRunning
	}
	wf.Status.Message = fmt.Sprintf("retrying step %s (%d/%d)", stepID, retries, maxRetries)
	log.FromContext(ctx).Info("workflow step scheduled for auto-retry",
		"workflow", wf.Name, "step", stepID, "retry", retries, "maxRetries", maxRetries, "task", taskName)
	return true
}

func (c *WorkflowController) upsertStepStatus(wf *agentflowiov1alpha1.Workflow, id string, phase agentflowiov1alpha1.TaskPhase, taskName, message string) {
	c.markStepStatus(wf, id, phase, taskName, message, "", nil)
}

func (c *WorkflowController) markStepRetry(wf *agentflowiov1alpha1.Workflow, id string, retries int32, message string) {
	now := metav1.Now()
	for i := range wf.Status.StepStatuses {
		if wf.Status.StepStatuses[i].ID == id {
			wf.Status.StepStatuses[i].Phase = ""
			wf.Status.StepStatuses[i].Retries = retries
			wf.Status.StepStatuses[i].TaskName = ""
			wf.Status.StepStatuses[i].Message = message
			wf.Status.StepStatuses[i].FinishedAt = &now
			return
		}
	}
	wf.Status.StepStatuses = append(wf.Status.StepStatuses, agentflowiov1alpha1.WorkflowStepStatus{
		ID:        id,
		Retries:   retries,
		Message:   message,
		StartedAt: &now,
	})
}

func (c *WorkflowController) markStepStatus(wf *agentflowiov1alpha1.Workflow, id string, phase agentflowiov1alpha1.TaskPhase, taskName, message, output string, qc *agentflowiov1alpha1.QualityCheck) {
	now := metav1.Now()
	for i := range wf.Status.StepStatuses {
		if wf.Status.StepStatuses[i].ID == id {
			retries := wf.Status.StepStatuses[i].Retries
			wf.Status.StepStatuses[i].Phase = phase
			wf.Status.StepStatuses[i].TaskName = taskName
			wf.Status.StepStatuses[i].Message = message
			if phase == agentflowiov1alpha1.TaskPhaseSucceeded {
				wf.Status.StepStatuses[i].Retries = 0
			} else {
				wf.Status.StepStatuses[i].Retries = retries
			}
			if output != "" {
				wf.Status.StepStatuses[i].Output = output
			}
			if qc != nil {
				wf.Status.StepStatuses[i].Score = qc.Score
			}
			if phase == agentflowiov1alpha1.TaskPhaseSucceeded || phase == agentflowiov1alpha1.TaskPhaseFailed {
				wf.Status.StepStatuses[i].FinishedAt = &now
			}
			return
		}
	}
	st := agentflowiov1alpha1.WorkflowStepStatus{
		ID:        id,
		Phase:     phase,
		TaskName:  taskName,
		Message:   message,
		Output:    output,
		StartedAt: &now,
	}
	if qc != nil {
		st.Score = qc.Score
	}
	wf.Status.StepStatuses = append(wf.Status.StepStatuses, st)
}

func removeString(items []string, target string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item != target {
			out = append(out, item)
		}
	}
	return out
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func (c *WorkflowController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentflowiov1alpha1.Workflow{}).
		Watches(
			&agentflowiov1alpha1.Task{},
			handler.EnqueueRequestsFromMapFunc(c.mapTaskToWorkflow),
		).
		Complete(c)
}

func (c *WorkflowController) mapTaskToWorkflow(ctx context.Context, obj client.Object) []reconcile.Request {
	task, ok := obj.(*agentflowiov1alpha1.Task)
	if !ok {
		return nil
	}
	wfName := task.Labels["agentflow.io/workflow"]
	if wfName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: wfName, Namespace: task.Namespace}}}
}
