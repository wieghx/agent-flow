package metrics

import (
	"errors"
	"testing"
	"time"
)

func TestRecordAIRequestAndSnapshot(t *testing.T) {
	RecordAIRequest("worker", "deepseek-chat", nil, 120*time.Millisecond, 100, 50)
	RecordAIRequest("worker", "deepseek-chat", errors.New("boom"), time.Second, 0, 0)

	sum, err := GatherSummary()
	if err != nil {
		t.Fatal(err)
	}
	if sum.AIRequestsOK < 1 || sum.AIRequestsError < 1 {
		t.Fatalf("unexpected ai request totals: %+v", sum)
	}
	if sum.AITokensPrompt < 100 || sum.AITokensCompletion < 50 {
		t.Fatalf("unexpected token totals: %+v", sum)
	}
}

func TestWorkflowReconcileResult(t *testing.T) {
	if got := WorkflowReconcileResult(errors.New("x"), 0); got != "error" {
		t.Fatalf("got %q", got)
	}
	if got := WorkflowReconcileResult(nil, time.Second); got != "requeue" {
		t.Fatalf("got %q", got)
	}
	if got := WorkflowReconcileResult(nil, 5*time.Second); got != "requeue" {
		t.Fatalf("got %q", got)
	}
	if got := WorkflowReconcileResult(nil, 0); got != "ok" {
		t.Fatalf("got %q", got)
	}
}
