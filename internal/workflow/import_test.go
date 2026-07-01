package workflow

import "testing"

func TestParseImportedNovel(t *testing.T) {
	raw := "前言\n\n第1章 风起\n内容一\n\n第2章 雨落\n内容二"
	chs, err := ParseImportedNovel(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(chs) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chs))
	}
	if chs[0].Num != 1 || chs[1].Num != 2 {
		t.Fatalf("unexpected nums: %#v", chs)
	}
}

func TestMergeImportedOutline(t *testing.T) {
	chs := []ImportedChapter{{Num: 1, Title: "章1", Content: "很长的第一章正文内容用于生成摘要"}}
	outline := MergeImportedOutline("测试书", chs)
	if outline.Title != "测试书" || len(outline.Chapters) != 1 {
		t.Fatalf("unexpected outline: %#v", outline)
	}
}