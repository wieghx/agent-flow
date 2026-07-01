package api

import (
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	wfengine "agent-flow/internal/workflow"
)

func enrichNovelProgress(wf *agentflowiov1alpha1.Workflow, s *NovelSummary) {
	if wf == nil || s == nil {
		return
	}
	plotsDone, plotsWriting, plotsFailed := countPlotStepStats(wf)
	s.PlotsDone = plotsDone
	s.PlotsWriting = plotsWriting
	s.PlotsFailed = plotsFailed
	s.PipelineStage = derivePipelineStage(wf, s)
}

func countPlotStepStats(wf *agentflowiov1alpha1.Workflow) (done, writing, failed int) {
	for _, id := range wf.Status.CompletedSteps {
		if _, ok := wfengine.PlotNumFromStepID(id); ok {
			done++
		}
	}
	for _, st := range wf.Status.StepStatuses {
		if _, ok := wfengine.PlotNumFromStepID(st.ID); !ok {
			continue
		}
		switch st.Phase {
		case agentflowiov1alpha1.TaskPhaseRunning, agentflowiov1alpha1.TaskPhasePending:
			writing++
		case agentflowiov1alpha1.TaskPhaseFailed:
			failed++
		}
	}
	return done, writing, failed
}

func completedStepSetFromWF(wf *agentflowiov1alpha1.Workflow) map[string]bool {
	out := make(map[string]bool, len(wf.Status.CompletedSteps))
	for _, id := range wf.Status.CompletedSteps {
		out[id] = true
	}
	return out
}

func derivePipelineStage(wf *agentflowiov1alpha1.Workflow, s *NovelSummary) string {
	if wf.Status.Phase == agentflowiov1alpha1.WorkflowPhaseSucceeded {
		return "done"
	}

	completed := completedStepSetFromWF(wf)
	current := strings.TrimSpace(wf.Status.CurrentStep)
	chapterCount := s.ChapterCount
	if chapterCount <= 0 {
		chapterCount = wfengine.OutlineChapterCount(wf)
	}
	if chapterCount <= 0 {
		chapterCount = wfengine.IntParam(wf.Spec.Params, "chapterCount", 0)
	}

	template := wf.Spec.Template
	if template == "novel-import-deconstruct" || wfengine.BoolParam(wf.Spec.Params, "importedNovel", false) {
		if !completed["import-deconstruct"] {
			return "deconstruct"
		}
		if !completed["import-rag-index"] {
			return "rag-index"
		}
	}

	if chapterCount > 0 && s.ChaptersDone >= chapterCount {
		if completed["merge"] || wf.Status.Phase == agentflowiov1alpha1.WorkflowPhaseSucceeded {
			return "done"
		}
		return "merge"
	}

	if strings.HasPrefix(current, "plot-") {
		return "plots"
	}
	if strings.HasPrefix(current, "chapter-") {
		return "chapters"
	}
	if current == "merge" {
		return "merge"
	}
	if current == "plots" {
		return "plots"
	}
	if current == "chapters" {
		return "chapters"
	}

	if wfengine.PlotsStageActive(wf) && chapterCount > 0 {
		if s.PlotsDone < chapterCount && s.PlotsDone <= s.ChaptersDone {
			return "plots"
		}
		if s.ChaptersDone < chapterCount {
			return "chapters"
		}
	} else if chapterCount > 0 && s.ChaptersDone < chapterCount {
		if strings.HasPrefix(current, "chapter-") || completed["outline-refine"] || completed["outline"] {
			return "chapters"
		}
	}

	switch current {
	case "import-deconstruct":
		return "deconstruct"
	case "import-rag-index":
		return "rag-index"
	case "style-bible", "outline-refine", "outline-merge", "outline", "outline-skeleton":
		return "outline"
	}
	if strings.HasPrefix(current, "outline-vol-") || current == "historical-research" {
		return "outline"
	}

	if !completed["outline"] && !completed["outline-merge"] && !completed["import-deconstruct"] {
		return "outline"
	}
	if wfengine.PlotsStageActive(wf) {
		return "plots"
	}
	if chapterCount > 0 {
		return "chapters"
	}
	return "outline"
}