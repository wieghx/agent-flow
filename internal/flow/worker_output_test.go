package flow

import (
	"strings"
	"testing"

	wfengine "agent-flow/internal/workflow"
)

func TestShouldBypassPVCOutputSegmentedChapter(t *testing.T) {
	instruction := "写本章正文\n" + wfengine.SegmentDirectiveBlock + "\nsegments: 5\nsegmentWords: 500\n"
	input := State{
		WorkerInstruction: instruction,
		MonitorTaskType:   TaskTypeNovelChapter,
	}
	if !shouldBypassPVCOutput(input) {
		t.Fatal("segmented chapter should bypass PVC cache")
	}
}

func TestShouldBypassPVCOutputRegularChapter(t *testing.T) {
	input := State{
		WorkerInstruction: "你是小说作者。目标字数约: 2500 字",
		MonitorTaskType:   TaskTypeNovelChapter,
	}
	if shouldBypassPVCOutput(input) {
		t.Fatal("non-segmented chapter may use PVC cache")
	}
}

func TestShouldBypassPVCOutputTeamChapter(t *testing.T) {
	input := State{
		WorkerInstruction: "你是小说执笔者。目标字数约: 800 字",
		MonitorTaskType:   TaskTypeNovelChapterTeam,
	}
	if !shouldBypassPVCOutput(input) {
		t.Fatal("team chapter must always bypass PVC cache")
	}
}

func TestShouldBypassPVCOutputWithFeedback(t *testing.T) {
	input := State{
		WorkerInstruction: "你是小说作者。目标字数约: 2500 字",
		MonitorTaskType:   TaskTypeNovelChapter,
		MonitorFeedback:   "质量评分 60/72（未通过）",
	}
	if !shouldBypassPVCOutput(input) {
		t.Fatal("QC retry with feedback must bypass PVC cache")
	}
}

func TestPvcChapterOutputTooShortTeamChapter(t *testing.T) {
	input := State{
		WorkerInstruction: "目标字数约: 800 字",
		MonitorTaskType:   TaskTypeNovelChapterTeam,
	}
	short := strings.Repeat("短", 50)
	if !pvcChapterOutputTooShort(input, short) {
		t.Fatal("team chapter short cache should be rejected")
	}
}

func TestPvcChapterOutputTooShort(t *testing.T) {
	input := State{
		WorkerInstruction: "目标字数约: 2500 字",
		MonitorTaskType:   TaskTypeNovelChapter,
	}
	short := strings.Repeat("短", 50)
	if !pvcChapterOutputTooShort(input, short) {
		t.Fatal("very short chapter cache should be rejected")
	}
	long := strings.Repeat("荒岛求生剧情推进，人物对话自然衔接。", 80)
	if pvcChapterOutputTooShort(input, long) {
		t.Fatal("long enough chapter cache should be accepted")
	}
}

func TestPvcChapterOutputTooShortNonChapter(t *testing.T) {
	input := State{
		WorkerInstruction: "implement hello world",
		MonitorTaskType:   TaskTypeCode,
	}
	if pvcChapterOutputTooShort(input, "tiny") {
		t.Fatal("non-chapter tasks should not use chapter length gate")
	}
}
