package workflow

import (
	"fmt"
	"regexp"
	"strings"

	"agent-flow/internal/prompts"
)

const (
	DefaultSegmentWords    = 500
	MinSegmentWords        = 300
	DefaultSegmentCount    = 5
	MinSegmentRunesFloor   = 120
	SegmentMinRunesPercent = 24 // 24% of segmentWords; 500-word segments require >= 120 runes
	SegmentDirectiveBlock  = "【分段写作配置】"
)

// MinSegmentRunes returns the minimum prose length required for one chapter segment.
func MinSegmentRunes(segmentWords int) int {
	minSeg := segmentWords * SegmentMinRunesPercent / 100
	if minSeg < MinSegmentRunesFloor {
		return MinSegmentRunesFloor
	}
	return minSeg
}

var segmentMetaLine = regexp.MustCompile(`^segments:\s*(\d+)\s*$`)
var segmentWordsLine = regexp.MustCompile(`^segmentWords:\s*(\d+)\s*$`)

// SegmentModeEnabled reports whether chapter generation should use segmented writing.
// Very short chapters (< MinWordsForSegmentMode) use single-pass generation for coherence.
func SegmentModeEnabled(params map[string]string, wordsPerChapter int) bool {
	if params == nil {
		params = map[string]string{}
	}
	raw := strings.ToLower(strings.TrimSpace(params["chapterSegmentMode"]))
	if raw == "false" || raw == "0" || raw == "off" {
		return false
	}
	if wordsPerChapter > 0 && wordsPerChapter < MinWordsForSegmentMode {
		return false
	}
	return true
}

// SegmentWordsPerPart returns target runes per segment.
func SegmentWordsPerPart(params map[string]string, wordsPerChapter int) int {
	if n := IntParam(params, "segmentWords", 0); n >= MinSegmentWords {
		return n
	}
	if wordsPerChapter > 0 {
		n := wordsPerChapter / DefaultSegmentCount
		if n < MinSegmentWords {
			return MinSegmentWords
		}
		if n > 800 {
			return 800
		}
		return n
	}
	return DefaultSegmentWords
}

// SegmentCount returns how many segments compose one chapter.
func SegmentCount(params map[string]string, wordsPerChapter int) int {
	if n := IntParam(params, "chapterSegments", 0); n > 0 {
		return n
	}
	segWords := SegmentWordsPerPart(params, wordsPerChapter)
	if wordsPerChapter <= 0 {
		return DefaultSegmentCount
	}
	n := (wordsPerChapter + segWords - 1) / segWords
	if n < 3 {
		return 3
	}
	if n > 8 {
		return 8
	}
	return n
}

// AppendSegmentDirectives adds machine-readable segment config to chapter instructions.
func AppendSegmentDirectives(instruction string, segments, segmentWords int) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(instruction))
	b.WriteString("\n\n")
	b.WriteString(SegmentDirectiveBlock)
	b.WriteString("\n")
	fmt.Fprintf(&b, "segments: %d\n", segments)
	fmt.Fprintf(&b, "segmentWords: %d\n", segmentWords)
	b.WriteString("说明: 系统将按段调用你撰写本章，每段须与上一段自然衔接，文风、人称与时态保持一致。")
	return b.String()
}

// ParseSegmentConfig reads segment settings embedded in worker instruction.
func ParseSegmentConfig(instruction string) (segments int, segmentWords int, ok bool) {
	segments = DefaultSegmentCount
	segmentWords = DefaultSegmentWords
	block := instruction
	if idx := strings.Index(instruction, SegmentDirectiveBlock); idx >= 0 {
		block = instruction[idx:]
	}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if m := segmentMetaLine.FindStringSubmatch(line); len(m) == 2 {
			segments = IntParam(map[string]string{"v": m[1]}, "v", DefaultSegmentCount)
		}
		if m := segmentWordsLine.FindStringSubmatch(line); len(m) == 2 {
			segmentWords = IntParam(map[string]string{"v": m[1]}, "v", DefaultSegmentWords)
		}
	}
	if segments <= 0 || segmentWords <= 0 {
		return 0, 0, false
	}
	return segments, segmentWords, strings.Contains(instruction, SegmentDirectiveBlock)
}

// BuildSegmentInstruction renders prompt for one chapter segment.
func BuildSegmentInstruction(baseInstruction string, segmentIndex, totalSegments, segmentWords int, priorTail, openingSample string) string {
	return prompts.BuildSegmentInstruction(baseInstruction, segmentIndex, totalSegments, segmentWords, priorTail, openingSample)
}

// StitchChapterSegments joins segment prose into one chapter.
func StitchChapterSegments(segments []string) string {
	var parts []string
	for i, seg := range segments {
		seg = strings.TrimSpace(seg)
		if seg == "" {
			continue
		}
		if i > 0 {
			seg = stripLeadingChapterTitle(seg)
		}
		parts = append(parts, seg)
	}
	return strings.Join(parts, "\n\n")
}

func stripSegmentDirectiveBlock(instruction string) string {
	idx := strings.Index(instruction, SegmentDirectiveBlock)
	if idx < 0 {
		return instruction
	}
	return strings.TrimSpace(instruction[:idx])
}

var leadingChapterTitle = regexp.MustCompile(`(?m)^#\s*第\s*\d+\s*章[^\n]*\n+`)

func stripLeadingChapterTitle(text string) string {
	text = strings.TrimSpace(text)
	for i := 0; i < 2; i++ {
		trimmed := strings.TrimSpace(leadingChapterTitle.ReplaceAllString(text, ""))
		if trimmed == text {
			break
		}
		text = trimmed
	}
	return text
}
