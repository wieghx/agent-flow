package workflow

import (
	"regexp"
	"strconv"
	"strings"
)

var chapterCountRE = regexp.MustCompile(`(\d+)\s*章`)

// DefaultNovelParams returns production-oriented defaults for novel-outline-chapters.
func DefaultNovelParams(chapterCount int) map[string]string {
	if chapterCount <= 0 {
		chapterCount = 100
	}
	return map[string]string{
		"teamMode":               "true",
		"chapterCount":           strconv.Itoa(chapterCount),
		"wordsPerChapter":        "2500",
		"chapterSegmentMode":     "true",
		"chapterSegments":        "5",
		"segmentWords":           "500",
		"contextChapters":        "8",
		"volumeSize":             "25",
		"arcInterval":            "10",
		"qualityThreshold":       "78",
		"taskMaxRetries":         "6",
		"maxParallel":            "8",
		"chapterPipeline":        "8",
		"stepMaxRetries":         "3",
		"stepRetryBaseDelaySec":  "30",
		"stepRetryMaxDelaySec":   "300",
		"pauseOnStepFail":        "false",
		"threeStage":             "true",
		"ragEnabled":             "true",
		"ragTopK":                "5",
		"plotWords":              "1000",
	}
}

// MergeParams overlays override onto base (override wins).
func MergeParams(base, override map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		if strings.TrimSpace(v) != "" {
			out[k] = v
		}
	}
	return out
}

// ExtractChapterCountFromText parses phrases like "100章" from user text.
func ExtractChapterCountFromText(text string) int {
	m := chapterCountRE.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// LooksLikeNovelIntent reports whether the user message is requesting a multi-chapter novel.
func LooksLikeNovelIntent(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{"小说", "长篇", "章节", "逐章", "大纲", "100章", "五十章", "写一本", "写部"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return ExtractChapterCountFromText(text) > 0
}

// BoolParam reads a boolean workflow parameter.
func BoolParam(params map[string]string, key string, fallback bool) bool {
	raw, ok := params[key]
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}