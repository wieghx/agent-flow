package architecture

import (
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	wfengine "agent-flow/internal/workflow"
)

func TestCountActiveWorkflowTasksIgnoresClearedSteps(t *testing.T) {
	c := &WorkflowController{}
	wf := &agentflowiov1alpha1.Workflow{
		Status: agentflowiov1alpha1.WorkflowStatus{
			StepStatuses: []agentflowiov1alpha1.WorkflowStepStatus{
				{ID: "chapter-001", Phase: agentflowiov1alpha1.TaskPhasePending},
				{ID: "chapter-002", Phase: ""},
				{ID: "chapter-003", Phase: agentflowiov1alpha1.TaskPhaseRunning},
			},
		},
	}
	if got := c.countActiveWorkflowTasks(wf); got != 2 {
		t.Fatalf("countActiveWorkflowTasks() = %d, want 2", got)
	}

	wfengine.ReconcileStaleStepStatuses(wf, map[string]agentflowiov1alpha1.TaskPhase{
		"chapter-003": agentflowiov1alpha1.TaskPhaseRunning,
	})
	if got := c.countActiveWorkflowTasks(wf); got != 1 {
		t.Fatalf("after stale reconcile countActiveWorkflowTasks() = %d, want 1", got)
	}
}

func TestNonBlockingFailedStepEligibleForRedispatch(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Params: map[string]string{"chapterCount": "20"},
			Execution: agentflowiov1alpha1.WorkflowExecution{
				StepMaxRetries: 3,
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CompletedSteps: []string{
				"outline",
				"chapter-01", "chapter-02", "chapter-03", "chapter-04",
			},
			FailedSteps: []string{"chapter-15"},
			StepStatuses: []agentflowiov1alpha1.WorkflowStepStatus{
				{ID: "chapter-15", Phase: agentflowiov1alpha1.TaskPhaseFailed, Retries: 3},
			},
		},
	}
	missing := wfengine.MissingChapterNumbers(wf)
	if wfengine.FailedStepBlocksProgressForStep(wf, "chapter-15", missing) {
		t.Fatal("chapter-15 failure should not block when earlier chapters are missing")
	}
	if wfengine.StepFailureExhausted(wf, "chapter-15") != true {
		t.Fatal("expected workflow retries exhausted for chapter-15")
	}
}