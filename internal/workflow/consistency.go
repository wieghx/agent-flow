package workflow

import (
	"fmt"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

// BuildConsistencyMonitorContext assembles cross-chapter reference material for Monitor.
func BuildConsistencyMonitorContext(wf *agentflowiov1alpha1.Workflow, stepID string) string {
	chapterNum, ok := ChapterNumFromStepID(stepID)
	if !ok || chapterNum <= 0 {
		return ""
	}

	outline, err := LoadOutline(wf)
	if err != nil || outline == nil {
		return ""
	}

	width := ChapterPaddingWidth(len(outline.Chapters))
	contextWindow := IntParam(wf.Spec.Params, "contextChapters", 5)
	ctxBundle := BuildChapterContext(wf, chapterNum, width, contextWindow)

	var b strings.Builder
	fmt.Fprintf(&b, "【跨章一致性检查】第%d章\n", chapterNum)
	fmt.Fprintf(&b, "书名: %s\n", outline.Title)

	if chars := FormatCharacters(outline); chars != "" {
		b.WriteString("\n设定人物（名称/性格不得矛盾）:\n")
		b.WriteString(chars)
		b.WriteString("\n")
	}

	for _, ch := range outline.Chapters {
		if ch.Num == chapterNum {
			fmt.Fprintf(&b, "\n本章大纲要求:\n标题: %s\n梗概: %s\n", ch.Title, ch.Summary)
			break
		}
	}

	if ctxBundle.ArcSummaries != "" {
		b.WriteString("\n已完成故事弧:\n")
		b.WriteString(ctxBundle.ArcSummaries)
		b.WriteString("\n")
	}
	if ctxBundle.StorySoFar != "" {
		b.WriteString("\n更早剧情:\n")
		b.WriteString(ctxBundle.StorySoFar)
		b.WriteString("\n")
	}
	if ctxBundle.RecentSummaries != "" {
		b.WriteString("\n近几章摘要:\n")
		b.WriteString(ctxBundle.RecentSummaries)
		b.WriteString("\n")
	}
	if ctxBundle.PreviousEnding != "" {
		b.WriteString("\n上一章结尾（本章必须自然衔接，不得矛盾）:\n")
		b.WriteString(ctxBundle.PreviousEnding)
		b.WriteString("\n")
	}

	words := IntParam(wf.Spec.Params, "wordsPerChapter", 3000)
	fmt.Fprintf(&b, "\n目标字数约: %d\n", words)
	return b.String()
}

// MinChapterLength estimates minimum acceptable chapter length in runes.
func MinChapterLength(wordsPerChapter int) int {
	if wordsPerChapter <= 0 {
		wordsPerChapter = 3000
	}
	// Chinese ~1 rune per word; accept 20% of target as hard floor.
	min := wordsPerChapter * 2 / 10
	if min < 200 {
		return 200
	}
	return min
}
