package workflow

import "testing"

func TestParseVolumeChapterRangeFromInstruction(t *testing.T) {
	instr := "为第4卷（第76-100章）生成详细章节大纲\n章节范围: 第76章 到 第100章"
	start, end, ok := ParseVolumeChapterRangeFromInstruction(instr)
	if !ok || start != 76 || end != 100 {
		t.Fatalf("ParseVolumeChapterRangeFromInstruction() = %d-%d %v", start, end, ok)
	}
}

func TestValidateVolumeOutlineComplete(t *testing.T) {
	raw := `{"volume":1,"chapters":[{"num":1,"title":"a","summary":"s"},{"num":2,"title":"b","summary":"s2"}]}`
	if err := ValidateVolumeOutline(raw, 1, 2); err != nil {
		t.Fatalf("ValidateVolumeOutline() err = %v", err)
	}
}

func TestValidateVolumeOutlineTruncated(t *testing.T) {
	raw := `{"volume":4,"chapters":[{"num":76,"title":"a","summary":"s"},{"num":77,"title":"b","summary":"s2"}]}`
	err := ValidateVolumeOutline(raw, 76, 100)
	if err == nil {
		t.Fatal("expected error for incomplete volume outline")
	}
}

func TestValidateVolumeOutlineBrokenJSON(t *testing.T) {
	raw := `{"volume":4,"chapters":[{"num":76,"title":"a","summary":"s"},{"`
	if err := ValidateVolumeOutline(raw, 76, 100); err == nil {
		t.Fatal("expected error for broken JSON")
	}
}
