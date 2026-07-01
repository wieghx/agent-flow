package flow

import (
	"os"
	"strings"
	"testing"
)

func TestRunRuleChecks_teamChapter_withProtagonist(t *testing.T) {
	instr := `【设定圣经 — 全书必须遵守】
主角（姓名不可更改）: 苏晴
你是小说执笔者。`
	out := strings.Repeat("苏晴", 5) + "站在废弃工厂。"
	res := RunRuleChecks(instr, out, TaskTypeNovelChapterTeam)
	for _, iss := range res.Issues {
		if iss == "no_main_character_mentioned" || iss == "bible_protagonist_missing" {
			t.Fatalf("unexpected issue %s score=%d issues=%v", iss, res.Score, res.Issues)
		}
	}
}

func TestExtractLabeledSection_falsePositive_prevChapter(t *testing.T) {
	instr := `你是小说执笔者。根据设定圣经、大纲、人物、故事弧与上一章结尾撰写本章初稿。
主角（姓名不可更改）: 苏晴`
	got := extractLabeledSection(instr, "上一章结尾")
	if got != "" {
		t.Fatalf("false positive prev section: %q", got)
	}
}

func TestRunRuleChecks_realChapter1Artifact(t *testing.T) {
	instr, err := os.ReadFile("/tmp/instr.txt")
	if err != nil {
		t.Skip("no artifact")
	}
	out, err := os.ReadFile("/tmp/ch1.txt")
	if err != nil {
		t.Skip("no artifact")
	}
	res := RunRuleChecks(string(instr), string(out), TaskTypeNovelChapterTeam)
	t.Logf("score=%d issues=%v", res.Score, res.Issues)
	for _, iss := range res.Issues {
		if iss == "no_main_character_mentioned" || iss == "bible_protagonist_missing" || iss == "continuity_break" {
			t.Fatalf("unexpected %s", iss)
		}
	}
}
