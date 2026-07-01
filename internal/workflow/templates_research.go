package workflow

import (
	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

func historicalResearchStep(quality int32) agentflowiov1alpha1.WorkflowStep {
	return agentflowiov1alpha1.WorkflowStep{
		ID:   "historical-research",
		Name: "历史背景调研（MCP 联网）",
		Type: agentflowiov1alpha1.WorkflowStepTypeAITask,
		TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
			WorkerInstruction: "PLACEHOLDER — resolved at runtime via BuildHistoricalResearchInstruction",
			MCPMode:           true,
			QualityThreshold:  quality,
			MonitorTaskType:   "general",
		},
		Output: agentflowiov1alpha1.WorkflowStepOutput{Path: ResearchArtifact, Format: "markdown"},
	}
}