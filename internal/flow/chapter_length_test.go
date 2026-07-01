package flow

import "testing"

func TestParseTargetWordsFromInstruction(t *testing.T) {
	cases := map[string]int{
		"目标字数约: 2000 字":                  2000,
		"目标字数约 800 字":                    800,
		"目标字数: 约2500字":                   2500,
		"\n目标字数约: 1500 字（正文不少于 450 字）\n": 1500,
	}
	for instr, want := range cases {
		if got := ParseTargetWordsFromInstruction(instr); got != want {
			t.Fatalf("instruction %q: got %d want %d", instr, got, want)
		}
	}
}

func TestLooksTruncated(t *testing.T) {
	if LooksTruncated("短。") {
		t.Fatal("complete short sentence should not be truncated")
	}
	if !LooksTruncated("林默利用地形优势，将雷虎逼") {
		t.Fatal("mid-sentence ending should be truncated")
	}
	if LooksTruncated("完整的一段叙事，人物离开现场。") {
		t.Fatal("complete ending should not be truncated")
	}
}

func TestMinChapterRunes(t *testing.T) {
	if got := MinChapterRunes(2500); got != 500 {
		t.Fatalf("MinChapterRunes(2500) = %d, want 500 (20%%)", got)
	}
}
