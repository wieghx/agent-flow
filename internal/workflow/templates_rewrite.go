package workflow

import (
	"strconv"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

func chapterRewriteSpec(prompt string, params map[string]string) agentflowiov1alpha1.WorkflowSpec {
	chapterNum := RewriteChapterNum(params)
	quality := int32(IntParam(params, "qualityThreshold", 75))
	layer := RewriteLayer(params)
	layerLabel := "正文"
	if layer == RewriteLayerPlot {
		layerLabel = "剧情"
	}

	rewrite := agentflowiov1alpha1.WorkflowStep{
		ID:   "rewrite",
		Name: "重写第" + strconv.Itoa(chapterNum) + "章" + layerLabel,
		Type: agentflowiov1alpha1.WorkflowStepTypeAITask,
		TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
			WorkerInstruction: prompt,
			QualityThreshold:  quality,
			MonitorTaskType:   RewriteMonitorType(params),
		},
		Output: agentflowiov1alpha1.WorkflowStepOutput{
			Path:   RewriteOutputPath(params),
			Format: "markdown",
		},
	}

	outlineSync := agentflowiov1alpha1.WorkflowStep{
		ID:        "outline-sync",
		Name:      "同步章节梗概",
		Type:      agentflowiov1alpha1.WorkflowStepTypeAITask,
		DependsOn: []string{"rewrite"},
		TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
			WorkerInstruction: BuildOutlineSyncInstruction(chapterNum),
			QualityThreshold:  70,
			MonitorTaskType:   "novel-outline-refine",
		},
		Output: agentflowiov1alpha1.WorkflowStepOutput{Path: "outline.json", Format: "json"},
	}

	return agentflowiov1alpha1.WorkflowSpec{
		Prompt:   prompt,
		Template: "novel-chapter-rewrite",
		Params:   params,
		Execution: agentflowiov1alpha1.WorkflowExecution{
			Mode:            ExecutionModeSequential,
			MaxParallel:     1,
			StepMaxRetries:  DefaultStepMaxRetries,
			PauseOnStepFail: true,
			AutoApprove:     true,
		},
		Workspace: agentflowiov1alpha1.WorkflowWorkspace{PVC: "task-outputs"},
		Steps:     []agentflowiov1alpha1.WorkflowStep{rewrite, outlineSync},
	}
}