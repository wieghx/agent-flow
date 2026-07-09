package api

import (
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

func TestDerivePipelineStagePlots(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Template: "novel-team-chapters",
			Params:   map[string]string{"threeStage": "true", "chapterCount": "10"},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CurrentStep:    "plot-03",
			CompletedSteps: []string{"outline", "outline-refine", "plot-01", "plot-02"},
		},
	}
	s := &NovelSummary{ChapterCount: 10, PlotsDone: 2}
	if got := derivePipelineStage(wf, s); got != "plots" {
		t.Fatalf("expected plots, got %q", got)
	}
}

func TestDerivePipelineStageImport(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Template: "novel-import-deconstruct",
			Params:   map[string]string{"chapterCount": "5"},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CurrentStep: "import-deconstruct",
		},
	}
	s := &NovelSummary{ChapterCount: 5}
	if got := derivePipelineStage(wf, s); got != "deconstruct" {
		t.Fatalf("expected deconstruct, got %q", got)
	}
}
