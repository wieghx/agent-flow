package workflow

import "testing"

func TestExtractBibleProtagonists_parentheticalLabel(t *testing.T) {
	instr := "【设定圣经】\n主角（姓名不可更改）: 苏晴\n"
	got := ExtractBibleProtagonists(instr)
	if len(got) != 1 || got[0] != "苏晴" {
		t.Fatalf("got %v", got)
	}
}

func TestExtractBibleProtagonists_ignoresPOVLine(t *testing.T) {
	instr := `人称: 第三人称限知视角，严格锁定苏晴的感官体验，读者仅知晓主角所知的信息
主角（姓名不可更改）: 苏晴`
	got := ExtractBibleProtagonists(instr)
	if len(got) != 1 || got[0] != "苏晴" {
		t.Fatalf("got %v", got)
	}
}
