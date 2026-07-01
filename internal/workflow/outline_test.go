package workflow

import "testing"

func TestParseOutlineJSON(t *testing.T) {
	raw := `说明文字
{
  "title": "荒岛求生",
  "synopsis": "一群人在荒岛上挣扎求生",
  "characters": [{"name":"林峰","role":"主角","trait":"冷静"}],
  "chapters": [
    {"num":2,"title":"风暴","summary":"遭遇风暴"},
    {"num":1,"title":"启程","summary":"出海"}
  ]
}`
	outline, err := ParseOutlineJSON(raw)
	if err != nil {
		t.Fatalf("ParseOutlineJSON failed: %v", err)
	}
	if outline.Title != "荒岛求生" {
		t.Fatalf("unexpected title: %s", outline.Title)
	}
	if len(outline.Chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(outline.Chapters))
	}
	if outline.Chapters[0].Num != 1 {
		t.Fatalf("chapters should be sorted by num, got %d", outline.Chapters[0].Num)
	}
}

func TestChapterStepIDAndParse(t *testing.T) {
	id := ChapterStepID("chapter", 3, 2)
	if id != "chapter-03" {
		t.Fatalf("unexpected step id: %s", id)
	}
	id100 := ChapterStepID("chapter", 100, 3)
	if id100 != "chapter-100" {
		t.Fatalf("unexpected step id for chapter 100: %s", id100)
	}
	num, ok := ChapterNumFromStepID(id)
	if !ok || num != 3 {
		t.Fatalf("unexpected chapter num: %d ok=%v", num, ok)
	}
	if n, ok := ChapterNumFromStepID("outline-vol-01"); ok {
		t.Fatalf("outline-vol-01 should not parse as chapter, got %d", n)
	}
}

func TestSummarizeChapter(t *testing.T) {
	content := "这是一段很长的章节正文，需要被截断以便作为上下文摘要使用。"
	got := SummarizeChapter(content, 10)
	if len(got) > 13 {
		t.Fatalf("summary too long: %q", got)
	}
}
