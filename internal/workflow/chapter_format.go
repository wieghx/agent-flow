package workflow

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ChapterPaddingWidth returns zero-padding width for chapter file/step IDs.
func ChapterPaddingWidth(chapterCount int) int {
	if chapterCount <= 0 {
		return 2
	}
	width := len(strconv.Itoa(chapterCount))
	if width < 2 {
		return 2
	}
	return width
}

// ChapterStepID returns canonical step id for a chapter number.
func ChapterStepID(prefix string, num int, width int) string {
	if prefix == "" {
		prefix = "chapter"
	}
	if width <= 0 {
		width = 2
	}
	return fmt.Sprintf("%s-%0*d", prefix, width, num)
}

// ChapterFileName returns workspace chapter markdown filename.
func ChapterFileName(num, width int) string {
	if width <= 0 {
		width = 2
	}
	return fmt.Sprintf("chapter-%0*d.md", width, num)
}

// ChapterSummaryFileName returns workspace chapter summary filename.
func ChapterSummaryFileName(num, width int) string {
	if width <= 0 {
		width = 2
	}
	return fmt.Sprintf("chapter-%0*d.summary", width, num)
}

// ParseChapterFileNum extracts chapter number from chapter-003.md style names.
func ParseChapterFileNum(name string) (int, bool) {
	base := strings.TrimSuffix(name, ".md")
	base = strings.TrimSuffix(base, ".summary")
	if !strings.HasPrefix(base, "chapter-") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(base, "chapter-"))
	if err != nil {
		return 0, false
	}
	return n, true
}

// SortChapterFiles sorts chapter-*.md filenames by numeric chapter order.
func SortChapterFiles(files []string) {
	sort.Slice(files, func(i, j int) bool {
		ni, okI := ParseChapterFileNum(files[i])
		nj, okJ := ParseChapterFileNum(files[j])
		if okI && okJ {
			return ni < nj
		}
		return files[i] < files[j]
	})
}
