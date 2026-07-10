package workflow

import (
	"fmt"
	"strconv"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/prompts"
)

const defaultArcInterval = 10

// DefaultArcInterval picks arc summary cadence from workflow params.
func DefaultArcInterval(params map[string]string, chapterCount int) int {
	if raw := strings.TrimSpace(params["arcInterval"]); raw != "" {
		if raw == "0" {
			return 0
		}
		return IntParam(params, "arcInterval", 0)
	}
	if chapterCount > defaultVolumeThreshold {
		return defaultArcInterval
	}
	return 0
}

// ArcRange returns inclusive chapter range for an arc ending at endChapter.
func ArcRange(endChapter, interval int) (start, end int) {
	if interval <= 0 {
		return 1, endChapter
	}
	end = endChapter
	start = endChapter - interval + 1
	if start < 1 {
		start = 1
	}
	return start, end
}

// ArcStepID returns workflow step id like arc-010.
func ArcStepID(endChapter, width int) string {
	if width <= 0 {
		width = 2
	}
	return fmt.Sprintf("arc-%0*d", width, endChapter)
}

// ArcFileName returns workspace path like arcs/arc-001-010.md.
func ArcFileName(start, end, width int) string {
	if width <= 0 {
		width = 2
	}
	return fmt.Sprintf("arcs/arc-%0*d-%0*d.md", width, start, width, end)
}

// ArcEndFromStepID parses end chapter from arc-010.
func ArcEndFromStepID(stepID string) (int, bool) {
	if !strings.HasPrefix(stepID, "arc-") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(stepID, "arc-"))
	if err != nil {
		return 0, false
	}
	return n, true
}

// BuildArcSummaryInstruction creates prompt for arc recap task.
func BuildArcSummaryInstruction(wf *agentflowiov1alpha1.Workflow, outline *NovelOutline, start, end, width int) string {
	return prompts.BuildArcSummaryInstruction(wf.Spec.Prompt, start, end, width)
}

// LoadArcSummaries reads completed arc recap files before a chapter number.
func LoadArcSummaries(wf *agentflowiov1alpha1.Workflow, beforeNum, width int) string {
	if beforeNum <= 1 {
		return ""
	}
	interval := DefaultArcInterval(wf.Spec.Params, OutlineChapterCount(wf))
	if interval <= 0 {
		return ""
	}

	var parts []string
	for end := interval; end < beforeNum; end += interval {
		start, arcEnd := ArcRange(end, interval)
		path := ArcFileName(start, arcEnd, width)
		content, err := ReadArtifact(wf, path)
		if err != nil || strings.TrimSpace(content) == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("【第%d-%d章故事弧】\n%s", start, arcEnd, strings.TrimSpace(content)))
	}
	return strings.Join(parts, "\n\n")
}
