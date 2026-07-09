package flow

import (
	"strings"
	"testing"
)

func TestDetectTaskType(t *testing.T) {
	cases := map[string]string{
		"写一首七言绝句":                     TaskTypePoetry,
		"implement hello world in go": TaskTypeCode,
		"总结这篇文章":                      TaskTypeGeneral,
		"为第4卷（第76-100章）生成详细章节大纲":      TaskTypeNovelVolumeOutline,
	}
	for input, want := range cases {
		if got := DetectTaskType(input); got != want {
			t.Fatalf("DetectTaskType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRunRuleChecksEmptyOutput(t *testing.T) {
	result := RunRuleChecks("写诗", "", TaskTypePoetry)
	if result.Passed || result.Score != 0 {
		t.Fatalf("empty output should fail, got %+v", result)
	}
}

func TestRunRuleChecksPoetryStructure(t *testing.T) {
	output := "碧波荡漾映天晴，\n轻舟已过万重岭。\n风起荷香满池溢，\n月明人静听蝉鸣。"
	result := RunRuleChecks("写一首七言绝句", output, TaskTypePoetry)
	if result.Score < 50 {
		t.Fatalf("expected reasonable poetry score, got %+v", result)
	}
}

func TestParseMonitorResultJSON(t *testing.T) {
	raw := `{"score": 82, "passed": true, "feedback": "不错", "issues": [], "dimensions": {"completeness": 35, "accuracy": 25, "quality": 22}}`
	eval, err := ParseMonitorResult(raw, 70)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !eval.Passed || eval.Score != 82 {
		t.Fatalf("unexpected eval: %+v", eval)
	}
	if eval.Dimensions == nil || eval.Dimensions.Completeness != 35 {
		t.Fatalf("dimensions missing: %+v", eval.Dimensions)
	}
}

func TestParseMonitorResultPassAlias(t *testing.T) {
	raw := `{"score": 65, "pass": true, "feedback": "合格"}`
	eval, err := ParseMonitorResult(raw, 60)
	if err != nil || !eval.Passed {
		t.Fatalf("expected pass alias to work, got %+v err=%v", eval, err)
	}
}

func TestRunRuleChecksNovelChapter(t *testing.T) {
	instruction := "【跨章一致性检查】第2章\n设定人物（名称/性格不得矛盾）:\n- 林峰（主角）: 冷静\n目标字数约: 2500"
	short := "太短"
	result := RunRuleChecks(instruction, short, TaskTypeNovelChapter)
	if result.Passed {
		t.Fatalf("short chapter should fail: %+v", result)
	}

	long := strings.Repeat("林峰在岛上探索，发现淡水源。", 100)
	result = RunRuleChecks(instruction, long, TaskTypeNovelChapter)
	if result.Score < 40 {
		t.Fatalf("expected better score for valid chapter, got %+v", result)
	}
}

func TestBuildMonitorSystemPromptNovelChapter(t *testing.T) {
	prompt := BuildMonitorSystemPrompt(TaskTypeNovelChapter, 72, "")
	if !contains(prompt, "人物一致性") || !contains(prompt, "衔接性") {
		t.Fatalf("novel chapter prompt missing rubric: %s", prompt)
	}
}

func TestBuildLightMonitorSystemPrompt(t *testing.T) {
	prompt := BuildLightMonitorSystemPrompt(72)
	if !contains(prompt, "衔接") || !contains(prompt, "文笔") {
		t.Fatalf("light prompt missing rubric: %s", prompt)
	}
}

func TestResolveMonitorTier(t *testing.T) {
	if got := ResolveMonitorTier(TaskTypeNovelChapter, 0, false, false); got != MonitorTierLight {
		t.Fatalf("expected light, got %s", got)
	}
	if got := ResolveMonitorTier(TaskTypeNovelChapter, 1, false, false); got != MonitorTierFull {
		t.Fatalf("retry should escalate to full, got %s", got)
	}
	if got := ResolveMonitorTier(TaskTypeNovelChapter, 0, true, false); got != MonitorTierFull {
		t.Fatalf("arc boundary should be full, got %s", got)
	}
	if got := ResolveMonitorTier(TaskTypeNovelChapter, 0, false, true); got != MonitorTierFull {
		t.Fatalf("first chapter should be full, got %s", got)
	}
	if got := ResolveMonitorTier(TaskTypeNovelStyleBible, 0, false, false); got != MonitorTierFull {
		t.Fatalf("style bible should be full, got %s", got)
	}
	if got := ResolveMonitorTier(TaskTypeNovelChapterTeam, 0, false, false); got != MonitorTierLight {
		t.Fatalf("team chapter default should be light, got %s", got)
	}
	if got := ResolveMonitorTier(TaskTypeNovelOutline, 0, false, false); got != MonitorTierFull {
		t.Fatalf("outline should be full, got %s", got)
	}
}

func TestSanitizeMonitorFeedbackChainOfThought(t *testing.T) {
	raw := `Let's plan the score. Wait, checking dimensions.
严重偏离设定。主角姓名错误（林默 vs 苏晴），剧情完全不符。
{"score": 15, "feedback": "严重偏离设定。主角姓名错误。", "issues": ["character_rename"]}`
	got := sanitizeMonitorFeedback(raw, []string{"character_rename", "no_main_character_mentioned"})
	if contains(got, "Let's plan") || contains(got, "Wait, checking") {
		t.Fatalf("chain-of-thought leaked: %q", got)
	}
	if !contains(got, "苏晴") && !contains(got, "主角") {
		t.Fatalf("expected actionable Chinese feedback: %q", got)
	}
}

func TestFormatRetryFeedback(t *testing.T) {
	feedback := FormatRetryFeedback(&EvalResult{
		Score:      55,
		Feedback:   "押韵不工整",
		Issues:     []string{"rhyme_issue"},
		Dimensions: &EvalDimensions{Completeness: 20, Accuracy: 15, Quality: 20},
	}, 70)
	if feedback == "" || !contains(feedback, "55") {
		t.Fatalf("feedback should include score: %q", feedback)
	}
}

func TestBuildMonitorSystemPromptNovelOutline(t *testing.T) {
	prompt := BuildMonitorSystemPrompt(TaskTypeNovelOutline, 70, "")
	if !contains(prompt, "故事性") || !contains(prompt, "结构") {
		t.Fatalf("novel outline prompt missing rubric: %s", prompt)
	}
}

func TestBuildMonitorSystemPromptNovelOutlineRefine(t *testing.T) {
	prompt := BuildMonitorSystemPrompt(TaskTypeNovelOutlineRefine, 70, "")
	if !contains(prompt, "改进度") || !contains(prompt, "结构") {
		t.Fatalf("novel outline refine prompt missing rubric: %s", prompt)
	}
}

func TestBuildMonitorSystemPromptNovelOutlineSkeleton(t *testing.T) {
	prompt := BuildMonitorSystemPrompt(TaskTypeNovelOutlineSkeleton, 70, "")
	if !contains(prompt, "volumes") || !contains(prompt, "覆盖") {
		t.Fatalf("skeleton prompt missing rubric: %s", prompt)
	}
}

func TestRunRuleChecksNovelOutlineSkeleton(t *testing.T) {
	instruction := "全书共 4 章，分为 2 卷"
	valid := `{"title":"书","synopsis":"简介","characters":[],"volumes":[{"num":1,"title":"卷1","startChapter":1,"endChapter":2,"theme":"t","summary":"s"},{"num":2,"title":"卷2","startChapter":3,"endChapter":4,"theme":"t","summary":"s"}]}`
	result := RunRuleChecks(instruction, valid, TaskTypeNovelOutlineSkeleton)
	if result.Score < 90 {
		t.Fatalf("valid skeleton should score high, got %+v", result)
	}
	gapped := `{"title":"书","synopsis":"简介","characters":[],"volumes":[{"num":1,"title":"卷1","startChapter":1,"endChapter":2,"theme":"t","summary":"s"},{"num":2,"title":"卷2","startChapter":4,"endChapter":4,"theme":"t","summary":"s"}]}`
	result = RunRuleChecks(instruction, gapped, TaskTypeNovelOutlineSkeleton)
	if len(result.Issues) == 0 {
		t.Fatalf("gapped volumes should fail rules, got %+v", result)
	}
}

func TestResolveMonitorTierNovelOutlineRefine(t *testing.T) {
	if got := ResolveMonitorTier(TaskTypeNovelOutlineRefine, 0, false, false); got != MonitorTierFull {
		t.Fatalf("outline refine should be full, got %s", got)
	}
}

func TestRunRuleChecksNovelOutline(t *testing.T) {
	validJSON := `{"title":"测试书","synopsis":"简介","chapters":[{"num":1,"title":"章1","summary":"s1"}]}`
	result := RunRuleChecks("生成大纲", validJSON, TaskTypeNovelOutline)
	if result.Score < 60 {
		t.Fatalf("valid outline should score reasonably, got %+v", result)
	}

	invalidJSON := "这不是JSON"
	result = RunRuleChecks("生成大纲", invalidJSON, TaskTypeNovelOutline)
	if result.Passed {
		t.Fatalf("invalid JSON should fail, got %+v", result)
	}
}

func TestTryRuleOnlyJSONBypassOutlineRefine(t *testing.T) {
	valid := `{"title":"星尘纪元","synopsis":"简介","characters":[{"name":"林澈"}],"chapters":[{"num":1,"title":"章1","summary":"s1"}]}`
	rule := RunRuleChecks("审查并改进大纲", valid, TaskTypeNovelOutlineRefine)
	if rule.Score < 75 {
		t.Fatalf("rule score too low: %+v", rule)
	}
	got, ok := tryRuleOnlyJSONBypass(TaskTypeNovelOutlineRefine, valid, "审查并改进大纲", rule, 75)
	if !ok || got == nil || !got.Passed {
		t.Fatalf("expected rule-only bypass, ok=%v got=%+v", ok, got)
	}
	if got.CheckMethod != CheckMethodRule || got.Feedback == "" {
		t.Fatalf("unexpected bypass result: %+v", got)
	}
}

func TestTryRuleOnlyJSONBypassSkeleton(t *testing.T) {
	instruction := "全书共 2 章，分为 1 卷"
	valid := `{"title":"书","synopsis":"简介","characters":[],"volumes":[{"num":1,"title":"卷1","startChapter":1,"endChapter":2,"theme":"t","summary":"s"}]}`
	rule := RunRuleChecks(instruction, valid, TaskTypeNovelOutlineSkeleton)
	got, ok := tryRuleOnlyJSONBypass(TaskTypeNovelOutlineSkeleton, valid, instruction, rule, 70)
	if !ok || got == nil || !got.Passed {
		t.Fatalf("expected skeleton bypass, ok=%v got=%+v", ok, got)
	}
}
