package workflow

import (
	"sort"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

// MissingChapterNumbers returns chapter nums present in outline but not yet completed.
func MissingChapterNumbers(wf *agentflowiov1alpha1.Workflow) []int {
	count := OutlineChapterCount(wf)
	if count <= 0 {
		return nil
	}
	width := ChapterPaddingWidth(count)
	completed := completedStepSet(wf)

	var missing []int
	for n := 1; n <= count; n++ {
		id := ChapterStepID("chapter", n, width)
		if !completed[id] {
			missing = append(missing, n)
		}
	}
	return missing
}

// PrioritizeBackfillSteps moves missing chapter steps to the front so gaps are filled first.
func PrioritizeBackfillSteps(ready []ResolvedStep, missing []int) []ResolvedStep {
	if len(ready) <= 1 || len(missing) == 0 {
		return ready
	}

	missingSet := make(map[int]bool, len(missing))
	for _, n := range missing {
		missingSet[n] = true
	}

	var gaps, rest []ResolvedStep
	for _, step := range ready {
		num, ok := ChapterNumFromStepID(step.ID)
		if ok && missingSet[num] {
			gaps = append(gaps, step)
			continue
		}
		rest = append(rest, step)
	}
	if len(gaps) == 0 {
		return ready
	}

	sort.Slice(gaps, func(i, j int) bool {
		ni, _ := ChapterNumFromStepID(gaps[i].ID)
		nj, _ := ChapterNumFromStepID(gaps[j].ID)
		return ni < nj
	})
	return append(gaps, rest...)
}

// ReconcileStaleStepStatuses clears Pending/Running step slots when the backing task is gone or terminal.
func ReconcileStaleStepStatuses(wf *agentflowiov1alpha1.Workflow, livePhases map[string]agentflowiov1alpha1.TaskPhase) bool {
	completed := completedStepSet(wf)
	changed := false
	for i := range wf.Status.StepStatuses {
		st := &wf.Status.StepStatuses[i]
		if st.Phase != agentflowiov1alpha1.TaskPhaseRunning && st.Phase != agentflowiov1alpha1.TaskPhasePending {
			continue
		}
		phase, ok := livePhases[st.ID]
		if ok && (phase == agentflowiov1alpha1.TaskPhaseRunning || phase == agentflowiov1alpha1.TaskPhasePending) {
			continue
		}
		if completed[st.ID] {
			st.Phase = agentflowiov1alpha1.TaskPhaseSucceeded
		} else {
			st.Phase = ""
			st.TaskName = ""
			st.Message = "stale step reset, re-dispatching"
		}
		changed = true
	}
	return changed
}

// ClearStepDispatchState resets workflow step status so it can be dispatched again.
func ClearStepDispatchState(wf *agentflowiov1alpha1.Workflow, stepID string) {
	for i := range wf.Status.StepStatuses {
		if wf.Status.StepStatuses[i].ID == stepID {
			wf.Status.StepStatuses[i].Phase = ""
			wf.Status.StepStatuses[i].TaskName = ""
			wf.Status.StepStatuses[i].Message = ""
			wf.Status.StepStatuses[i].FinishedAt = nil
			return
		}
	}
}

// FailedStepBlocksProgressForStep reports whether a failed step blocks workflow progress.
func FailedStepBlocksProgressForStep(wf *agentflowiov1alpha1.Workflow, stepID string, missing []int) bool {
	num, ok := ChapterNumFromStepID(stepID)
	if !ok {
		return true
	}
	if len(missing) == 0 {
		return true
	}
	return num == missing[0]
}

// failedStepBlocksProgress reports whether a permanently failed step should pause the workflow.
// Only the earliest missing chapter failure is blocking; later chapter failures should not
// stop pipeline backfill of other ready steps.
func failedStepBlocksProgress(wf *agentflowiov1alpha1.Workflow, step ResolvedStep, missing []int) bool {
	if len(missing) == 0 {
		return true
	}
	num, ok := ChapterNumFromStepID(step.ID)
	if !ok {
		return true
	}
	return num == missing[0]
}

// StepFailureExhausted reports whether workflow-level retries are exhausted for a step.
func StepFailureExhausted(wf *agentflowiov1alpha1.Workflow, stepID string) bool {
	if !StepAutoRetryEnabled(wf) {
		return true
	}
	return int(StepStatusRetries(wf, stepID)) >= StepMaxRetries(wf)
}
