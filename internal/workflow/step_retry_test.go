package workflow

import (
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStepReadyForRetryWaitsAfterFailure(t *testing.T) {
	finished := metav1.Now()
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Execution: agentflowiov1alpha1.WorkflowExecution{
				StepRetryBaseDelaySec: 120,
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			StepStatuses: []agentflowiov1alpha1.WorkflowStepStatus{
				{ID: "chapter-001", Retries: 1, FinishedAt: &finished},
			},
		},
	}
	if StepReadyForRetry(wf, "chapter-001") {
		t.Fatal("expected step to wait before retry")
	}
	if wait := StepRetryWaitRemaining(wf, "chapter-001"); wait <= 0 {
		t.Fatalf("expected positive wait, got %v", wait)
	}
}

func TestStepReadyForRetryAllowsFreshStep(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{}
	if !StepReadyForRetry(wf, "chapter-001") {
		t.Fatal("expected fresh step to be ready")
	}
}
