package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
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
	raw, err := ReadArtifact(wf, OutlineDraftPath)
	if err != nil {
		return ""
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	const maxRunes = 12000
	runes := []rune(raw)
	if len(runes) > maxRunes {
		raw = string(runes[:maxRunes]) + "\n…（初稿已截断，仅保留前段供对比）"
	}
	var b strings.Builder
	b.WriteString("【大纲初稿（精修前）】\n")
	b.WriteString(raw)
	b.WriteString("\n\n请对比上方初稿与下方精修产出，评估故事弧、人物设定与章节衔接是否改进；不得删减或合并章节。")
	return b.String()
}

func outlineDraftFilePath(wf *agentflowiov1alpha1.Workflow) string {
	return filepath.Join(WorkspacePath(wf), OutlineDraftPath)
}