package workflow

import (
	"strconv"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

// EnhanceStepInstruction builds runtime worker prompts for volume/arc steps.
func EnhanceStepInstruction(wf *agentflowiov1alpha1.Workflow, step ResolvedStep) string {
	if step.ID == "historical-research" {
		return BuildHistoricalResearchInstruction(wf)
	}
	if step.ID == "outline-refine" {
		_ = SnapshotOutlineDraft(wf)
		instr := step.TaskSpec.WorkerInstruction
		instr += "\n\n工作区 outline-draft.json 为精修前初稿备份，outline.json 为待改进版本；改进后仍输出到 outline.json。"
		if block := ResearchContextBlock(wf); block != "" {
			instr += block
		}
		return instr
	}
	if step.ID == "style-bible" {
		outline, err := LoadOutline(wf)
		if err != nil {
			return step.TaskSpec.WorkerInstruction
		}
		instr := BuildStyleBibleInstruction(wf.Spec.Prompt, outline)
		if block := ResearchContextBlock(wf); block != "" {
			instr += block
		}
		return instr
	}
	if step.ID == "outline-skeleton" {
		instr := step.TaskSpec.WorkerInstruction
		if block := ResearchContextBlock(wf); block != "" {
			instr += block
		}
		return instr
	}
	if volNum, ok := VolumeNumFromStepID(step.ID); ok {
		instr := enhanceVolumeInstruction(wf, volNum, step.TaskSpec.WorkerInstruction)
		if block := ResearchContextBlock(wf); block != "" {
			instr += block
		}
		return instr
	}
	if arcEnd, ok := ArcEndFromStepID(step.ID); ok {
		width := ChapterPaddingWidth(OutlineChapterCount(wf))
		interval := DefaultArcInterval(wf.Spec.Params, OutlineChapterCount(wf))
		start, end := ArcRange(arcEnd, interval)
		outline, _ := LoadOutline(wf)
		return BuildArcSummaryInstruction(wf, outline, start, end, width)
	}
	return ""
}

// PrepareWorkerInstruction returns the final worker prompt for a resolved step.
func PrepareWorkerInstruction(wf *agentflowiov1alpha1.Workflow, step ResolvedStep) string {
	if enhanced := EnhanceStepInstruction(wf, step); enhanced != "" {
		return enhanced
	}
	return step.TaskSpec.WorkerInstruction
}

// LoadOutline reads merged outline.json from workspace.
func LoadOutline(wf *agentflowiov1alpha1.Workflow) (*NovelOutline, error) {
	raw, err := ReadArtifact(wf, "outline.json")
	if err != nil {
		return nil, err
	}
	return ParseOutlineJSON(raw)
}

// VolumeNumFromStepID parses volume number from outline-vol-02.
func VolumeNumFromStepID(stepID string) (int, bool) {
	if !strings.HasPrefix(stepID, "outline-vol-") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(stepID, "outline-vol-"))
	if err != nil {
		return 0, false
	}
	return n, true
}

func enhanceVolumeInstruction(wf *agentflowiov1alpha1.Workflow, volNum int, fallback string) string {
	skeletonRaw, err := ReadArtifact(wf, "skeleton.json")
	if err != nil {
		return fallback
	}
	skeleton, err := ParseSkeletonJSON(skeletonRaw)
	if err != nil {
		return fallback
	}
	var volMeta *VolumeMeta
	for i := range skeleton.Volumes {
		if skeleton.Volumes[i].Num == volNum {
			volMeta = &skeleton.Volumes[i]
			break
		}
	}
	if volMeta == nil {
		return fallback
	}

	prevSummary := ""
	if volNum > 1 {
		prevRaw, err := ReadArtifact(wf, VolumeFileName(volNum-1))
		if err == nil {
			if prevVol, err := ParseVolumeOutlineJSON(prevRaw); err == nil {
				var lines []string
				for _, ch := range prevVol.Chapters {
					lines = append(lines, ch.Summary)
				}
				prevSummary = SummarizeArc(lines, 800)
			}
		}
	}
	return BuildVolumeOutlineInstruction(wf.Spec.Prompt, skeleton, *volMeta, prevSummary)
}
