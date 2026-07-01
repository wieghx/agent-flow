package workflow

import (
	"strconv"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

func importDeconstructSpec(prompt string, params map[string]string) agentflowiov1alpha1.WorkflowSpec {
	chapterCount := IntParam(params, "chapterCount", 10)
	quality := int32(IntParam(params, "qualityThreshold", 75))
	title := params["title"]
	if title == "" {
		title = prompt
	}
	if title == "" {
		title = "导入小说"
	}

	execution := agentflowiov1alpha1.WorkflowExecution{
		Mode:            ExecutionModeParallel,
		MaxParallel:     1,
		StepMaxRetries:  DefaultStepMaxRetries,
		PauseOnStepFail: true,
		AutoApprove:     true,
	}

	deconstruct := agentflowiov1alpha1.WorkflowStep{
		ID:   "import-deconstruct",
		Name: "拆书：提取人物与剧情纲要",
		Type: agentflowiov1alpha1.WorkflowStepTypeAITask,
		TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
			WorkerInstruction: ImportDeconstructInstruction(title, chapterCount),
			QualityThreshold:  quality,
			MonitorTaskType:   "novel-outline",
		},
		Output: agentflowiov1alpha1.WorkflowStepOutput{Path: "outline.json", Format: "json"},
	}

	ragIndex := agentflowiov1alpha1.WorkflowStep{
		ID:        "import-rag-index",
		Name:      "构建 RAG 剧情索引",
		Type:      agentflowiov1alpha1.WorkflowStepTypeMerge,
		DependsOn: []string{"import-deconstruct"},
		Output:    agentflowiov1alpha1.WorkflowStepOutput{Path: "rag/index.json", Format: "json"},
	}

	spec := agentflowiov1alpha1.WorkflowSpec{
		Prompt:    prompt,
		Template:  "novel-import-deconstruct",
		Params:    params,
		Execution: execution,
		Workspace: agentflowiov1alpha1.WorkflowWorkspace{PVC: "task-outputs"},
		Steps:     []agentflowiov1alpha1.WorkflowStep{deconstruct, ragIndex},
	}
	if BoolParam(params, "continueWriting", true) {
		writeParams := MergeParams(params, map[string]string{"importedNovel": "true", "teamMode": "true"})
		writing := novelProductionSpec(prompt, writeParams, "novel-team-chapters")
		for _, step := range writing.Steps {
			if step.ID == "plots" || step.ID == "chapters" || step.ID == "merge" || step.ID == "style-bible" {
				if step.ID == "style-bible" {
					step.DependsOn = []string{"import-rag-index"}
				}
				spec.Steps = append(spec.Steps, step)
			}
		}
	}
	return spec
}

// ImportNovelParams returns defaults for import workflow.
func ImportNovelParams(chapterCount int, title string) map[string]string {
	return map[string]string{
		"chapterCount":     strconv.Itoa(chapterCount),
		"title":            title,
		"qualityThreshold": "75",
		"ragEnabled":       "true",
		"ragSearchMode":    "hybrid",
		"importedNovel":    "true",
		"continueWriting":  "true",
		"threeStage":       "true",
		"teamMode":         "true",
	}
}