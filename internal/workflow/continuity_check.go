package workflow

import (
	"strings"
	"unicode"
)

// commonContinuityBigrams are frequent pairs that appear across unrelated scenes.
var commonContinuityBigrams = map[string]struct{}{
	"之后": {}, "然后": {}, "已经": {}, "一种": {}, "一切": {}, "当时": {},
	"如此": {}, "只是": {}, "并非": {}, "不能": {}, "可以": {}, "这个": {},
	"那个": {}, "自己": {}, "他们": {}, "我们": {}, "你们": {}, "什么": {},
	"没有": {}, "不是": {}, "知道": {}, "看着": {}, "心中": {}, "身上": {},
	"现在": {}, "一起": {}, "突然": {}, "竟然": {}, "渐渐": {}, "默默": {},
	"静静": {}, "缓缓": {}, "微微": {}, "仿佛": {}, "似乎": {}, "或许": {},
	"也许": {}, "不过": {}, "然而": {}, "因此": {}, "于是": {}, "只见": {},
	"只听": {}, "说道": {}, "开口": {}, "声音": {}, "目光": {}, "眼神": {},
}

// ContinuityWeak reports whether chapter opening poorly connects to previous ending.
func ContinuityWeak(previousEnding, chapterOpening string) bool {
	prev := strings.TrimSpace(previousEnding)
	open := strings.TrimSpace(chapterOpening)
	if prev == "" || open == "" {
		return false
	}
	prevRunes := []rune(prev)
	openRunes := []rune(open)
	if len(prevRunes) < 10 || len(openRunes) < 10 {
		return false
	}
	tail := string(prevRunes[max(0, len(prevRunes)-120):])
	head := string(openRunes[:min(len(openRunes), 300)])

	for _, w := range []string{"次日", "第二天", "当晚", "随后", "与此同时", "回到", "仍然", "依旧", "接着", "这时", "与此同时"} {
		if strings.Contains(head, w) {
			return false
		}
	}

	// Carry-over of leading named entity (usually protagonist) from previous ending.
	if lead := leadingHanRunes(tail, 4); len(lead) >= 2 {
		if strings.Contains(head, string(lead)) {
			return false
		}
		if len(lead) >= 2 && strings.Contains(head, string(lead[:2])) {
			return false
		}
	}

	if hanNGramShared(tail, head, 3) {
		return false
	}
	if hanNGramSharedFiltered(tail, head, 2, commonContinuityBigrams) {
		return false
	}
	return true
}

func leadingHanRunes(s string, max int) []rune {
	var out []rune
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			out = append(out, r)
			if len(out) >= max {
				break
			}
			continue
		}
		if len(out) > 0 {
			break
		}
	}
	return out
}

func hanNGramShared(a, b string, n int) bool {
	if n <= 0 {
		return false
	}
	ar := []rune(a)
	for i := 0; i+n <= len(ar); i++ {
		ok := true
		gram := ar[i : i+n]
		for _, r := range gram {
			if !unicode.Is(unicode.Han, r) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		if strings.Contains(b, string(gram)) {
			return true
		}
	}
	return false
}

func hanNGramSharedFiltered(a, b string, n int, skip map[string]struct{}) bool {
	if n <= 0 {
		return false
	}
	ar := []rune(a)
	for i := 0; i+n <= len(ar); i++ {
		ok := true
		gram := ar[i : i+n]
		for _, r := range gram {
			if !unicode.Is(unicode.Han, r) {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		sub := string(gram)
		if _, banned := skip[sub]; banned {
			continue
		}
		if strings.Contains(b, sub) {
			return true
		}
	}
	return false
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}