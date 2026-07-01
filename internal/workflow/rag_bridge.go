package workflow

import (
	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/rag"
)

// RebuildRAGIndex rebuilds and persists the workspace RAG index.
func RebuildRAGIndex(wf *agentflowiov1alpha1.Workflow) (*rag.Index, error) {
	return rag.RebuildIndexAt(WorkspacePath(wf))
}