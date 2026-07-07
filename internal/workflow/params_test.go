package workflow

import "testing"

func TestDefaultNovelParams(t *testing.T) {
	params := DefaultNovelParams(50)
	if params["chapterCount"] != "50" {
		t.Fatalf("chapterCount: %s", params["chapterCount"])
	}
	if params["maxParallel"] != "8" {
		t.Fatalf("maxParallel: %s", params["maxParallel"])
	}
	if params["qualityThreshold"] != "78" {
		t.Fatalf("qualityThreshold: %s", params["qualityThreshold"])
	}
	if params["pauseOnStepFail"] != "false" {
		t.Fatalf("pauseOnStepFail: %s", params["pauseOnStepFail"])
	}
}

func TestExtractChapterCountFromText(t *testing.T) {
	if n := ExtractChapterCountFromText("写一本100章的小说"); n != 100 {
		t.Fatalf("expected 100, got %d", n)
	}
	if n := ExtractChapterCountFromText("随便聊聊"); n != 0 {
		t.Fatalf("expected 0, got %d", n)
	}
}

func TestLooksLikeNovelIntent(t *testing.T) {
	if !LooksLikeNovelIntent("帮我写长篇小说") {
		t.Fatal("expected novel intent")
	}
	if LooksLikeNovelIntent("查询服务器状态") {
		t.Fatal("expected false")
	}
}

func TestMergeParams(t *testing.T) {
	base := map[string]string{"a": "1", "b": "2"}
	override := map[string]string{"b": "9", "c": "3"}
	merged := MergeParams(base, override)
	if merged["a"] != "1" || merged["b"] != "9" || merged["c"] != "3" {
		t.Fatalf("unexpected merge: %v", merged)
	}
}