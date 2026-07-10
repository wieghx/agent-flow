package workflow

import (
	"fmt"
	"os"
	"path/filepath"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/prompts"
)

const OutlineDraftPath = "outline-draft.json"

// SnapshotOutlineDraft copies outline.json to outline-draft.json once before refine.
// Retries keep the original draft for monitor comparison.
func SnapshotOutlineDraft(wf *agentflowiov1alpha1.Workflow) error {
	draftPath := outlineDraftFilePath(wf)
	if _, err := os.Stat(draftPath); err == nil {
		return nil
	}
	raw, err := ReadArtifact(wf, "outline.json")
	if err != nil {
		return fmt.Errorf("snapshot outline draft: %w", err)
	}
	return WriteArtifact(wf, OutlineDraftPath, raw)
}

// BuildOutlineRefineMonitorContext supplies the pre-refine outline for comparative QA.
func BuildOutlineRefineMonitorContext(wf *agentflowiov1alpha1.Workflow) string {
	return prompts.BuildOutlineRefineMonitorContext()
}

func outlineDraftFilePath(wf *agentflowiov1alpha1.Workflow) string {
	return filepath.Join(WorkspacePath(wf), OutlineDraftPath)
}
