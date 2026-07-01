package workflow

import (
	"fmt"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

func styleBibleStep(prompt string, dependsOn []string, quality int32) agentflowiov1alpha1.WorkflowStep {
	return agentflowiov1alpha1.WorkflowStep{
		ID:        "style-bible",
		Name:      "生成设定圣经",
		Type:      agentflowiov1alpha1.WorkflowStepTypeAITask,
		DependsOn: dependsOn,
		TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
			WorkerInstruction: prompt + `

读取工作区 outline.json，生成设定圣经 style_bible.json（严格 JSON，不要 markdown 代码块）。
设定圣经是全书的写作契约：主角姓名、人称、语体、禁忌项不得被后续章节违反。`,
			QualityThreshold: quality,
			MonitorTaskType:  "novel-style-bible",
		},
		Output: agentflowiov1alpha1.WorkflowStepOutput{Path: StyleBibleArtifact, Format: "json"},
	}
}

func teamChapterForeachInstruction(wordsPerChapter int, params map[string]string) string {
	segCount := SegmentCount(params, wordsPerChapter)
	segWords := SegmentWordsPerPart(params, wordsPerChapter)
	instruction := fmt.Sprintf(`你是小说执笔者（Draft Writer）。根据设定圣经、大纲、人物、故事弧与上一章结尾撰写本章初稿。
目标字数约: %d 字（正文不少于 %d 字，须以完整句子收束）。
必须落实本章梗概，与前文自然衔接；主角姓名、性格、时间线不得偏离设定圣经。
写足场景、对话与心理描写；允许初稿略有瑕疵，润色编辑会在之后统一文风。`, wordsPerChapter, MinChapterLength(wordsPerChapter))
	if SegmentModeEnabled(params, wordsPerChapter) {
		instruction += fmt.Sprintf(`
系统将分 %d 段生成后拼接；每段须与设定圣经及已写部分保持一致。`, segCount)
		instruction = AppendSegmentDirectives(instruction, segCount, segWords)
	}
	instruction += `
文笔须流畅凝练，对话与动作推动剧情；禁止水文、重复凑字、元评论与提纲句。
只输出章节正文（可含章节标题），不要解释写作过程。`
	return instruction
}

func chapterForeachDependsOn(team bool, outlineDep string) []string {
	if team {
		return []string{outlineDep, "style-bible"}
	}
	return []string{outlineDep}
}