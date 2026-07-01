package workflow

import "testing"

func TestSortChapterFilesNumeric(t *testing.T) {
	files := []string{"chapter-100.md", "chapter-02.md", "chapter-10.md", "chapter-01.md"}
	SortChapterFiles(files)
	want := []string{"chapter-01.md", "chapter-02.md", "chapter-10.md", "chapter-100.md"}
	for i := range want {
		if files[i] != want[i] {
			t.Fatalf("sort mismatch at %d: got %v want %v", i, files, want)
		}
	}
}

func TestChapterPaddingWidth(t *testing.T) {
	if ChapterPaddingWidth(5) != 2 {
		t.Fatalf("expected width 2 for 5 chapters")
	}
	if ChapterPaddingWidth(100) != 3 {
		t.Fatalf("expected width 3 for 100 chapters")
	}
}
