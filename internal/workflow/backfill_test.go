package workflow

import (
	"os"
	"path/filepath"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

func TestMissingChapterNumbers(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		Status: agentflowiov1alpha1.WorkflowStatus{
			WorkspacePath: root,
		},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Params: map[string]string{"chapterCount": "4"},
		},
	}
	outline := `{"title":"书","synopsis":"介","chapters":[{"num":1,"title":"1","summary":"s"},{"num":2,"title":"2","summary":"s"},{"num":3,"title":"3","summary":"s"},{"num":4,"title":"4","summary":"s"}]}`
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(outline), 0644); err != nil {
		t.Fatal(err)
	}
	wf.Status.CompletedSteps = []string{"chapter-01", "chapter-03"}

	missing := MissingChapterNumbers(wf)
	if len(missing) != 2 || missing[0] != 2 || missing[1] != 4 {
		t.Fatalf("unexpected missing chapters: %#v", missing)
	}
}

func TestReconcileStaleStepStatuses(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Status: agentflowiov1alpha1.WorkflowStatus{
			StepStatuses: []agentflowiov1alpha1.WorkflowStepStatus{
				{ID: "chapter-001", Phase: agentflowiov1alpha1.TaskPhasePending, Message: "task created"},
				{ID: "chapter-005", Phase: agentflowiov1alpha1.TaskPhaseRunning},
				{ID: "chapter-010", Phase: agentflowiov1alpha1.TaskPhaseRunning},
			},
		},
	}
	live := map[string]agentflowiov1alpha1.TaskPhase{
		"chapter-010": agentflowiov1alpha1.TaskPhaseRunning,
	}
	if !ReconcileStaleStepStatuses(wf, live) {
		t.Fatal("expected stale statuses to be reconciled")
	}
	if wf.Status.StepStatuses[0].Phase != "" {
		t.Fatalf("chapter-001 should be reset, got %q", wf.Status.StepStatuses[0].Phase)
	}
	if wf.Status.StepStatuses[1].Phase != "" {
		t.Fatalf("chapter-005 should be reset, got %q", wf.Status.StepStatuses[1].Phase)
	}
	if wf.Status.StepStatuses[2].Phase != agentflowiov1alpha1.TaskPhaseRunning {
		t.Fatalf("chapter-010 should stay running, got %q", wf.Status.StepStatuses[2].Phase)
	}
}

func TestFailedStepBlocksProgressForStep(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{}
	missing := []int{1, 5, 9}
	if !FailedStepBlocksProgressForStep(wf, "chapter-01", missing) {
		t.Fatal("earliest missing chapter failure should block")
	}
	if FailedStepBlocksProgressForStep(wf, "chapter-05", missing) {
		t.Fatal("later missing chapter failure should not block")
	}
	if !FailedStepBlocksProgressForStep(wf, "outline-merge", missing) {
		t.Fatal("non-chapter failures should block when missing chapters remain")
	}
}

func TestClearStepDispatchState(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Status: agentflowiov1alpha1.WorkflowStatus{
			StepStatuses: []agentflowiov1alpha1.WorkflowStepStatus{
				{
					ID:       "chapter-016",
					Phase:    agentflowiov1alpha1.TaskPhaseFailed,
					TaskName: "wf-demo-chapter-016",
					Message:  "too short",
				},
			},
		},
	}
	ClearStepDispatchState(wf, "chapter-016")
	st := wf.Status.StepStatuses[0]
	if st.Phase != "" || st.TaskName != "" || st.Message != "" {
		t.Fatalf("step should be cleared for redispatch, got phase=%q task=%q msg=%q", st.Phase, st.TaskName, st.Message)
	}
}

func TestReadyStepsAfterStaleReset(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Steps: []agentflowiov1alpha1.WorkflowStep{
				{ID: "outline", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
			},
			Params: map[string]string{"chapterCount": "4"},
			Execution: agentflowiov1alpha1.WorkflowExecution{
				Mode: ExecutionModeParallel,
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CompletedSteps: []string{"outline"},
			StepStatuses: []agentflowiov1alpha1.WorkflowStepStatus{
				{ID: "chapter-01", Phase: agentflowiov1alpha1.TaskPhasePending, Message: "task created"},
			},
		},
	}
	resolved := []ResolvedStep{
		{ID: "chapter-01", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
		{ID: "chapter-02", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
	}
	ready, err := ReadySteps(wf, resolved)
	if err != nil {
		t.Fatalf("ReadySteps failed: %v", err)
	}
	if len(ready) != 0 {
		t.Fatalf("pending zombie should block ready list before reconcile, got %#v", stepIDs(ready))
	}

	ReconcileStaleStepStatuses(wf, map[string]agentflowiov1alpha1.TaskPhase{})
	ready, err = ReadySteps(wf, resolved)
	if err != nil {
		t.Fatalf("ReadySteps after stale reset failed: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != "chapter-01" {
		t.Fatalf("expected chapter-01 back in ready list, got %#v", stepIDs(ready))
	}
}

func TestReconcileStaleStepStatusesKeepsCompleted(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Status: agentflowiov1alpha1.WorkflowStatus{
			CompletedSteps: []string{"chapter-03"},
			StepStatuses: []agentflowiov1alpha1.WorkflowStepStatus{
				{ID: "chapter-03", Phase: agentflowiov1alpha1.TaskPhasePending},
			},
		},
	}
	ReconcileStaleStepStatuses(wf, map[string]agentflowiov1alpha1.TaskPhase{})
	if wf.Status.StepStatuses[0].Phase != agentflowiov1alpha1.TaskPhaseSucceeded {
		t.Fatalf("completed step should be marked succeeded, got %q", wf.Status.StepStatuses[0].Phase)
	}
}

func TestPrioritizeBackfillSteps(t *testing.T) {
	ready := []ResolvedStep{
		{ID: "chapter-030"},
		{ID: "chapter-015"},
		{ID: "chapter-019"},
		{ID: "outline-merge", Type: agentflowiov1alpha1.WorkflowStepTypeMerge},
	}
	got := PrioritizeBackfillSteps(ready, []int{15, 19, 23})
	if len(got) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(got))
	}
	if got[0].ID != "chapter-015" || got[1].ID != "chapter-019" {
		t.Fatalf("expected gap chapters first, got %#v", stepIDs(got))
	}
	if got[2].ID != "chapter-030" {
		t.Fatalf("expected non-gap chapter after gaps, got %s", got[2].ID)
	}
}

func TestReadyStepsEarliestMissingFailureBlocks(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Steps: []agentflowiov1alpha1.WorkflowStep{
				{ID: "outline", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
			},
			Params:    map[string]string{"chapterCount": "2"},
			Execution: agentflowiov1alpha1.WorkflowExecution{StepMaxRetries: 3},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CompletedSteps: []string{"outline"},
			StepStatuses: []agentflowiov1alpha1.WorkflowStepStatus{
				{ID: "chapter-01", Phase: agentflowiov1alpha1.TaskPhaseFailed, Retries: 3, Message: "too short"},
			},
		},
	}
	resolved := []ResolvedStep{
		{ID: "chapter-01", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
	}
	_, err := ReadySteps(wf, resolved)
	if err == nil {
		t.Fatal("expected error when earliest missing chapter permanently failed")
	}
}

func TestReadyStepsLaterMissingFailureDoesNotBlock(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Steps: []agentflowiov1alpha1.WorkflowStep{
				{ID: "outline", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
				{ID: "chapters", Type: agentflowiov1alpha1.WorkflowStepTypeForeach, DependsOn: []string{"outline"}},
			},
			Params: map[string]string{"chapterCount": "20"},
			Execution: agentflowiov1alpha1.WorkflowExecution{
				Mode:            ExecutionModeParallel,
				ChapterMode:     ChapterModePipeline,
				ChapterPipeline: 4,
				StepMaxRetries:  3,
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CompletedSteps: []string{
				"outline",
				"chapter-01", "chapter-02", "chapter-03", "chapter-04",
				"chapter-06", "chapter-07", "chapter-08", "chapter-09",
				"chapter-10", "chapter-11", "chapter-12", "chapter-13",
				"chapter-14", "chapter-16", "chapter-17", "chapter-18",
			},
			StepStatuses: []agentflowiov1alpha1.WorkflowStepStatus{
				{ID: "chapter-15", Phase: agentflowiov1alpha1.TaskPhaseFailed, Retries: 3, Message: "too short"},
			},
		},
	}
	resolved := []ResolvedStep{
		{ID: "chapter-05", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
		{ID: "chapter-15", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
		{ID: "chapter-19", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
		{ID: "chapter-20", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
	}
	ready, err := ReadySteps(wf, resolved)
	if err != nil {
		t.Fatalf("later missing chapter failure should not block pipeline progress: %v", err)
	}
	if len(ready) == 0 || ready[0].ID != "chapter-05" {
		t.Fatalf("expected chapter-05 backfill first, got %#v", stepIDs(ready))
	}
}

func TestReadyStepsPrioritizesMissingChapter(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Steps: []agentflowiov1alpha1.WorkflowStep{
				{ID: "outline", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
				{ID: "chapters", Type: agentflowiov1alpha1.WorkflowStepTypeForeach, DependsOn: []string{"outline"}},
			},
			Params: map[string]string{"chapterCount": "4"},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CompletedSteps: []string{"outline", "chapter-01", "chapter-03", "chapter-04"},
		},
	}
	resolved := []ResolvedStep{
		{ID: "outline", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
		{ID: "chapter-01", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
		{ID: "chapter-02", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
		{ID: "chapter-03", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
		{ID: "chapter-04", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
	}
	ready, err := ReadySteps(wf, resolved)
	if err != nil {
		t.Fatalf("ReadySteps failed: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != "chapter-02" {
		t.Fatalf("expected chapter-02 backfill, got %#v", ready)
	}
}
