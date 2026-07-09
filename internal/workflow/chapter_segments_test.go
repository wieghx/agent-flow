package workflow

import (
	"strings"
	"testing"
)

func TestMinSegmentRunes(t *testing.T) {
	if got := MinSegmentRunes(500); got != 120 {
		t.Fatalf("MinSegmentRunes(500) = %d, want 120", got)
	}
	if got := MinSegmentRunes(400); got != 120 {
		t.Fatalf("MinSegmentRunes(400) = %d, want 120 (floor)", got)
	}
	if got := MinSegmentRunes(600); got != 144 {
		t.Fatalf("MinSegmentRunes(600) = %d, want 144", got)
	}
}

func TestSegmentCountAndWords(t *testing.T) {
	params := map[string]string{"chapterSegments": "4", "segmentWords": "600"}
	if got := SegmentCount(params, 2500); got != 4 {
		t.Fatalf("SegmentCount() = %d, want 4", got)
	}
	if got := SegmentWordsPerPart(params, 2500); got != 600 {
		t.Fatalf("SegmentWordsPerPart() = %d, want 600", got)
	}
}

func TestSegmentCountAuto(t *testing.T) {
	got := SegmentCount(nil, 2500)
	if got < 3 || got > 8 {
		t.Fatalf("auto SegmentCount() = %d, out of range", got)
	}
}

func TestParseSegmentConfig(t *testing.T) {
	instr := "base\n\n" + SegmentDirectiveBlock + "\nsegments: 5\nsegmentWords: 500\n"
	seg, words, ok := ParseSegmentConfig(instr)
	if !ok || seg != 5 || words != 500 {
		t.Fatalf("ParseSegmentConfig() = %d %d %v", seg, words, ok)
	}
}

func TestBuildSegmentInstructionContinuity(t *testing.T) {
	base := "书名: 测试\n当前章节: 第1章\n\n" + SegmentDirectiveBlock + "\nsegments: 3\nsegmentWords: 400\n"
	s2 := BuildSegmentInstruction(base, 2, 3, 400, "他转身离开了营地。", "夜色笼罩营地，风声如诉。")
	if !strings.Contains(s2, "正文不少于 120 字") {
		t.Fatalf("expected lowered segment minimum in prompt: %s", s2)
	}
	if !strings.Contains(s2, "第 2/3 段") {
		t.Fatalf("missing segment role: %s", s2)
	}
	if !strings.Contains(s2, "他转身离开了营地。") {
		t.Fatalf("missing prior tail: %s", s2)
	}
	if !strings.Contains(s2, "夜色笼罩营地") {
		t.Fatalf("missing opening sample: %s", s2)
	}
	if !strings.Contains(s2, "一致性锚点") {
		t.Fatalf("missing consistency anchor: %s", s2)
	}
	if strings.Contains(s2, SegmentDirectiveBlock) {
		t.Fatal("segment prompt should not include directive block")
	}
}

func TestStitchChapterSegments(t *testing.T) {
	parts := []string{
		"# 第 1 章 开端\n\n第一段内容。",
		"# 第 1 章 续\n\n第二段内容。",
		"第三段收束。",
	}
	got := StitchChapterSegments(parts)
	if strings.Contains(got, "续") {
		t.Fatalf("duplicate title not stripped: %s", got)
	}
	if !strings.Contains(got, "第一段内容") || !strings.Contains(got, "第三段收束") {
		t.Fatalf("stitched content incomplete: %s", got)
	}
}
