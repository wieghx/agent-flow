package flow

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"agent-flow/internal/ai"
	wfengine "agent-flow/internal/workflow"
)

type stubChapterWriter struct {
	replies []string
	calls   int
}

func (s *stubChapterWriter) WorkerChat(_ context.Context, _, userMessage string) (ai.ChatResult, error) {
	if s.calls >= len(s.replies) {
		return ai.ChatResult{}, fmt.Errorf("no more replies")
	}
	out := s.replies[s.calls]
	s.calls++
	if !strings.Contains(userMessage, "【本段写作任务】") {
		return ai.ChatResult{}, fmt.Errorf("missing segment task block")
	}
	return ai.ChatResult{Content: out, Usage: ai.TokenUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30}}, nil
}

func TestGenerateSegmentedChapter(t *testing.T) {
	segText := strings.Repeat("荒岛求生剧情推进，人物对话与心理描写。", 20)
	instruction := "你是小说作者。\n目标字数约: 2500 字\n\n" + wfengine.SegmentDirectiveBlock + "\nsegments: 3\nsegmentWords: 400\n"

	writer := &stubChapterWriter{
		replies: []string{segText, segText, segText},
	}
	out, usage, err := GenerateSegmentedChapter(context.Background(), writer, instruction, "")
	if err != nil {
		t.Fatalf("GenerateSegmentedChapter failed: %v", err)
	}
	if usage.TotalTokens != 90 {
		t.Fatalf("expected 90 total tokens, got %d", usage.TotalTokens)
	}
	if writer.calls != 3 {
		t.Fatalf("expected 3 segment calls, got %d", writer.calls)
	}
	if len([]rune(out)) < 500 {
		t.Fatalf("stitched chapter too short: %d runes", len([]rune(out)))
	}
}

func TestGenerateSegmentedChapterRetriesShortSegment(t *testing.T) {
	long := strings.Repeat("荒岛求生剧情推进，人物对话与心理描写。", 25)
	short := "太短。"
	instruction := "你是小说作者。\n目标字数约: 2500 字\n\n" + wfengine.SegmentDirectiveBlock + "\nsegments: 2\nsegmentWords: 400\n"

	writer := &stubChapterWriter{
		replies: []string{short, long, long},
	}
	out, _, err := GenerateSegmentedChapter(context.Background(), writer, instruction, "")
	if err != nil {
		t.Fatalf("GenerateSegmentedChapter failed: %v", err)
	}
	if writer.calls != 3 {
		t.Fatalf("expected 3 calls (1 retry + 2 segments), got %d", writer.calls)
	}
	if len([]rune(out)) < 400 {
		t.Fatalf("stitched chapter too short: %d runes", len([]rune(out)))
	}
}

func TestShouldUseSegmentedChapter(t *testing.T) {
	instr := "x\n" + wfengine.SegmentDirectiveBlock + "\nsegments: 2\nsegmentWords: 300\n"
	if !ShouldUseSegmentedChapter(instr, TaskTypeNovelChapter) {
		t.Fatal("expected segmented mode")
	}
	if ShouldUseSegmentedChapter("plain chapter prompt", TaskTypeNovelChapter) {
		t.Fatal("expected single-shot mode without directive block")
	}
}
