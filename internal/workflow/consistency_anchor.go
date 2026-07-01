package workflow

import (
	"fmt"
	"strings"
)

const (
	// MinWordsForSegmentMode disables segmented writing for very short chapters.
	MinWordsForSegmentMode = 1200
	// SegmentPriorTailRunes is how much prior segment text is passed for衔接.
	SegmentPriorTailRunes = 600
	// SegmentOpeningSampleRunes is the opening tone sample for later segments.
	SegmentOpeningSampleRunes = 220
)

// BuildConsistencyAnchor extracts a compact consistency block for every segment prompt.
func BuildConsistencyAnchor(instruction string) string {
	base := stripSegmentDirectiveBlock(instruction)
	lines := strings.Split(base, "\n")

	var title, chapter, summary string
	var charLines []string
	inChars := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case hasLabelPrefix(trimmed, "书名"):
			title = labelValue(trimmed, "书名")
		case hasLabelPrefix(trimmed, "当前章节"):
			chapter = labelValue(trimmed, "当前章节")
		case hasLabelPrefix(trimmed, "本章梗概"):
			summary = labelValue(trimmed, "本章梗概")
		case strings.Contains(trimmed, "主要人物") || strings.Contains(trimmed, "设定人物"):
			inChars = true
			continue
		}
		if inChars {
			if trimmed == "" || strings.HasPrefix(trimmed, "当前章节") || strings.HasPrefix(trimmed, "【") {
				inChars = false
				continue
			}
			if strings.HasPrefix(trimmed, "- ") {
				charLines = append(charLines, trimmed)
				if len(charLines) >= 5 {
					inChars = false
				}
			}
		}
	}

	if title == "" && chapter == "" && summary == "" && len(charLines) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("【一致性锚点 — 全章/全段必须遵守，不得偏离】\n")
	if title != "" {
		fmt.Fprintf(&b, "书名: %s\n", title)
	}
	if chapter != "" {
		fmt.Fprintf(&b, "章节: %s\n", chapter)
	}
	if summary != "" {
		fmt.Fprintf(&b, "本章梗概: %s\n", summary)
	}
	if len(charLines) > 0 {
		b.WriteString("人物（姓名/性格不得更改，禁止擅自换主角或引入未登记主角）:\n")
		for _, cl := range charLines {
			b.WriteString(cl)
			b.WriteString("\n")
		}
	}
	b.WriteString("叙事要求: 第三人称（若指令另有规定则从指令）、语体统一、段间自然衔接，禁止拼凑感与突兀跳剪。")
	return b.String()
}

func hasLabelPrefix(line, label string) bool {
	return strings.HasPrefix(line, label+":") || strings.HasPrefix(line, label+"：")
}

func labelValue(line, label string) string {
	for _, sep := range []string{label + ":", label + "："} {
		if strings.HasPrefix(line, sep) {
			return strings.TrimSpace(strings.TrimPrefix(line, sep))
		}
	}
	return ""
}