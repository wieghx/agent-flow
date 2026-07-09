package workflow

import (
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/rag"
)

// RebuildRAGIndex rebuilds and persists the workspace RAG index.
func RebuildRAGIndex(wf *agentflowiov1alpha1.Workflow) (*rag.Index, error) {
	return rag.RebuildIndexAt(WorkspacePath(wf))
}

// UpdateRAGIndexForStep incrementally updates RAG after a workflow step writes an artifact.
func UpdateRAGIndexForStep(wf *agentflowiov1alpha1.Workflow, stepID, relPath string) error {
	if wf == nil || !RAGEnabled(wf.Spec.Params) {
		return nil
	}
	root := WorkspacePath(wf)
	relPath = strings.TrimSpace(relPath)

	if stepID == "outline-sync" || isOutlineRAGStep(stepID, relPath) {
		return rag.RefreshOutlineAt(root)
	}
	if stepID == "rewrite" && relPath != "" {
		return rag.UpsertArtifactAt(root, relPath)
	}
	if relPath == "" {
		return nil
	}
	if _, ok := PlotNumFromStepID(stepID); ok {
		return rag.UpsertArtifactAt(root, relPath)
	}
	if _, ok := ChapterNumFromStepID(stepID); ok {
		return rag.UpsertArtifactAt(root, relPath)
	}
	return nil
}

func isOutlineRAGStep(stepID, relPath string) bool {
	if strings.HasSuffix(relPath, "outline.json") {
		return true
	}
	switch stepID {
	case "outline", "outline-refine", "import-deconstruct", "outline-merge":
		return true
	default:
		return false
	}
}
