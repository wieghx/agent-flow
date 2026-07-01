package flow

import (
	"fmt"
	"strings"
	"unicode"

	wfengine "agent-flow/internal/workflow"
)

// ParseTargetWordsFromInstruction extracts the per-chapter word target from worker instruction.
func ParseTargetWordsFromInstruction(instruction string) int {
	patterns := []string{
		"目标字数约: %d",
		"目标字数约 %d",
		"目标字数: 约%d字",
		"目标字数: 约%d",
	}
	for _, p := range patterns {
		var n int
		if _, err := fmt.Sscanf(extractLineContaining(instruction, "目标字数"), p, &n); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func extractLineContaining(text, keyword string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, keyword) {
			return strings.TrimSpace(line)
		}
	}
	return text
}

// MinChapterRunes returns the minimum acceptable chapter length in runes.
func MinChapterRunes(targetWords int) int {
	if targetWords <= 0 {
		return 400
	}
	// 至少达到目标字数的 20%，且不低于 workflow 的 20% 下限。
	min := targetWords * 20 / 100
	floor := wfengine.MinChapterLength(targetWords)
	if min < floor {
		min = floor
	}
	if min < 200 {
		min = 200
	}
	return min
}

// LooksTruncated reports whether prose ends mid-sentence (common with token limits).
func LooksTruncated(prose string) bool {
	prose = strings.TrimSpace(prose)
	runes := []rune(prose)
	if len(runes) < 12 {
		return false
	}
	last := runes[len(runes)-1]
	switch last {
	case '。', '！', '？', '…', '」', '』', '"', '”', '’':
		return false
	}
	// 以汉字/字母/逗号结尾且无收束标点，视为被截断。
	if unicode.Is(unicode.Han, last) || unicode.IsLetter(last) || last == '，' || last == '、' {
		return true
	}
	return false
}
