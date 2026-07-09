package workflow

import (
	"time"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/retry"
)

// StepRetryBackoff returns how long to wait before re-dispatching a failed step.
func StepRetryBackoff(wf *agentflowiov1alpha1.Workflow, attempt int) time.Duration {
	return retry.Backoff(attempt, StepRetryBaseDelaySec(wf), StepRetryMaxDelaySec(wf))
}

// StepReadyForRetry reports whether enough time has passed since the last failure.
func StepReadyForRetry(wf *agentflowiov1alpha1.Workflow, stepID string) bool {
	st := findStepStatus(wf, stepID)
	if st == nil || st.Retries == 0 {
		return true
	}
	if st.FinishedAt == nil {
		return true
	}
	return time.Since(st.FinishedAt.Time) >= StepRetryBackoff(wf, int(st.Retries))
}

// StepRetryWaitRemaining returns remaining wait time before a step may be retried.
func StepRetryWaitRemaining(wf *agentflowiov1alpha1.Workflow, stepID string) time.Duration {
	st := findStepStatus(wf, stepID)
	if st == nil || st.Retries == 0 || st.FinishedAt == nil {
		return 0
	}
	wait := StepRetryBackoff(wf, int(st.Retries)) - time.Since(st.FinishedAt.Time)
	if wait < 0 {
		return 0
	}
	return wait
}
