package flow

import (
	"strings"
	"unicode"
)

// ExtractChineseProse pulls the longest Chinese narrative block from noisy model output.
func ExtractChineseProse(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	blocks := splitProseBlocks(text)
	best := ""
	bestScore := 0
	for _, block := range blocks {
		score := chineseProseScore(block)
		if score > bestScore {
			bestScore = score
			best = block
		}
	}
	if bestScore >= 120 {
		return strings.TrimSpace(best)
	}
	return text
}

func splitProseBlocks(text string) []string {
	lines := strings.Split(text, "\n")
	var blocks []string
	var b strings.Builder

	flush := func() {
		if s := strings.TrimSpace(b.String()); s != "" {
			blocks = append(blocks, s)
		}
		b.Reset()
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		if isMetaLine(line) {
			flush()
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	flush()
	return blocks
}

func isMetaLine(line string) bool {
	if strings.HasPrefix(line, "*") || strings.HasPrefix(line, "```") {
		return true
	}
	lower := strings.ToLower(line)
	metaHints := []string{
		"wait,", "okay,", "decision:", "self-correction", "drafting",
		"review and edit", "word count", "system instruction", "user input",
		"re-evaluating", "hypothesis:", "final check", "let's ",
	}
	for _, h := range metaHints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

func chineseProseScore(text string) int {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}
	chinese := 0
	for _, r := range runes {
		if unicode.Is(unicode.Han, r) {
			chinese++
		}
	}
	if chinese < 80 {
		return 0
	}
	return chinese
}
