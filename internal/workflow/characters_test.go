package workflow

import "testing"

func TestExtractListedCharacterNames(t *testing.T) {
	section := `主要人物（必须保持一致）:
- 苏晴（记者）: 冷静
- 王叔（线人）: 谨慎

当前章节: 第1章`
	names := ExtractListedCharacterNames(section)
	if len(names) != 2 || names[0] != "苏晴" || names[1] != "王叔" {
		t.Fatalf("names = %v", names)
	}
}

func TestValidateCharacterPresence(t *testing.T) {
	instr := "主要人物:\n- 苏晴（记者）: 冷静\n\n当前章节: 第1章"
	ok, _ := ValidateCharacterPresence(instr, "苏晴走进雨夜。")
	if !ok {
		t.Fatal("expected character present")
	}
	ok, names := ValidateCharacterPresence(instr, "林默走在街头。")
	if ok || len(names) != 1 {
		t.Fatalf("expected missing character, names=%v", names)
	}
}

func TestSegmentModeDisabledForShortChapter(t *testing.T) {
	if SegmentModeEnabled(map[string]string{"chapterSegmentMode": "true"}, 800) {
		t.Fatal("800-word chapter should not use segment mode")
	}
	if !SegmentModeEnabled(map[string]string{"chapterSegmentMode": "true"}, 2500) {
		t.Fatal("2500-word chapter should use segment mode")
	}
}