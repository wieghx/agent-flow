package workflow

import "testing"

func TestParseOutlineJSON(t *testing.T) {
	raw := `{"title":"测试","synopsis":"简介","characters":[{"name":"林远","role":"主角","trait":"冷静"}],"chapters":[{"num":1,"title":"开端","summary":"故事开始"}]}`
	outline, err := ParseOutlineJSON(raw)
	if err != nil {
		t.Fatalf("ParseOutlineJSON failed: %v", err)
	}
	if outline.Title != "测试" || len(outline.Chapters) != 1 {
		t.Fatalf("unexpected outline: %+v", outline)
	}
}

func TestMarshalOutlineJSON(t *testing.T) {
	outline := &NovelOutline{
		Title:    "测试",
		Synopsis: "简介",
		Chapters: []ChapterOutline{{Num: 1, Title: "开端", Summary: "故事开始"}},
	}
	raw, err := MarshalOutlineJSON(outline)
	if err != nil {
		t.Fatalf("MarshalOutlineJSON failed: %v", err)
	}
	parsed, err := ParseOutlineJSON(raw)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}
	if parsed.Title != outline.Title || len(parsed.Chapters) != 1 {
		t.Fatalf("round-trip mismatch: %+v", parsed)
	}
}
