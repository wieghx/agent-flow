package workflow

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	volumeRangePattern1 = regexp.MustCompile(`第(\d+)-(\d+)章`)
	volumeRangePattern2 = regexp.MustCompile(`第(\d+)章\s*到\s*第(\d+)章`)
)

// ParseVolumeChapterRangeFromInstruction extracts expected chapter span from a volume outline prompt.
func ParseVolumeChapterRangeFromInstruction(instruction string) (start, end int, ok bool) {
	for _, re := range []*regexp.Regexp{volumeRangePattern1, volumeRangePattern2} {
		if m := re.FindStringSubmatch(instruction); len(m) == 3 {
			start = IntParam(map[string]string{"v": m[1]}, "v", 0)
			end = IntParam(map[string]string{"v": m[2]}, "v", 0)
			if start > 0 && end >= start {
				return start, end, true
			}
		}
	}
	return 0, 0, false
}

// ValidateVolumeOutline ensures per-volume outline JSON is complete and covers the requested span.
func ValidateVolumeOutline(raw string, startChapter, endChapter int) error {
	if startChapter <= 0 || endChapter < startChapter {
		return fmt.Errorf("invalid chapter range %d-%d", startChapter, endChapter)
	}
	vol, err := ParseVolumeOutlineJSON(raw)
	if err != nil {
		return err
	}
	expected := endChapter - startChapter + 1
	if len(vol.Chapters) != expected {
		return fmt.Errorf("volume outline has %d chapters, expected %d (chapters %d-%d)", len(vol.Chapters), expected, startChapter, endChapter)
	}
	for i, ch := range vol.Chapters {
		want := startChapter + i
		if ch.Num != want {
			return fmt.Errorf("chapter index %d has num=%d, expected %d", i, ch.Num, want)
		}
		if strings.TrimSpace(ch.Title) == "" {
			return fmt.Errorf("chapter %d missing title", ch.Num)
		}
		if strings.TrimSpace(ch.Summary) == "" {
			return fmt.Errorf("chapter %d missing summary", ch.Num)
		}
	}
	return nil
}
