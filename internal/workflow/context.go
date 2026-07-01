package workflow

import (
	"fmt"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

// BuildChapterContext assembles rolling context for long sequential novels.
func BuildChapterContext(wf *agentflowiov1alpha1.Workflow, beforeNum, width, window int) ContextBundle {
	if window <= 0 {
		window = 5
	}
	if beforeNum <= 1 {
		return ContextBundle{}
	}

	var storyLines []string
	var recentLines []string
	recentStart := beforeNum - window
	if recentStart < 1 {
		recentStart = 1
	}

	for num := 1; num < beforeNum; num++ {
		summaryPath := fmt.Sprintf("chapters/%s", ChapterSummaryFileName(num, width))
		summary, err := ReadArtifact(wf, summaryPath)
		if err != nil || strings.TrimSpace(summary) == "" {
			content, err := ReadArtifact(wf, fmt.Sprintf("chapters/%s", ChapterFileName(num, width)))
			if err != nil {
				continue
			}
			summary = SummarizeChapter(content, 300)
		}
		line := fmt.Sprintf("第%d章摘要: %s", num, strings.TrimSpace(summary))
		if num < recentStart {
			storyLines = append(storyLines, line)
			continue
		}
		recentLines = append(recentLines, line)
	}

	prevEnding := ""
	if content, err := ReadArtifact(wf, fmt.Sprintf("chapters/%s", ChapterFileName(beforeNum-1, width))); err == nil {
		prevEnding = ChapterEnding(content, 500)
	}

	return ContextBundle{
		ArcSummaries:    LoadArcSummaries(wf, beforeNum, width),
		StorySoFar:      SummarizeArc(storyLines, 1500),
		RecentSummaries: strings.Join(recentLines, "\n"),
		PreviousEnding:  prevEnding,
	}
}

// SummarizeArc compresses older chapter summaries into a bounded recap.
func SummarizeArc(lines []string, maxLen int) string {
	if len(lines) == 0 || maxLen <= 0 {
		return ""
	}
	joined := strings.Join(lines, "\n")
	if len([]rune(joined)) <= maxLen {
		return joined
	}
	runes := []rune(joined)
	return string(runes[:maxLen]) + "..."
}

// OutlineChapterCount returns chapter count from outline artifact or workflow params.
func OutlineChapterCount(wf *agentflowiov1alpha1.Workflow) int {
	raw, err := ReadArtifact(wf, "outline.json")
	if err == nil {
		if outline, err := ParseOutlineJSON(raw); err == nil && len(outline.Chapters) > 0 {
			return len(outline.Chapters)
		}
	}
	return IntParam(wf.Spec.Params, "chapterCount", 10)
}
