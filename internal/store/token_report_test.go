package store

import (
	"context"
	"path/filepath"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/ai"
	wfengine "agent-flow/internal/workflow"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildTokenReport(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "report.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = st.Close() }()
	s := st.(*SQLStore)

	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "novel-a", Namespace: "default"},
	}
	ctx := context.Background()
	outline := &wfengine.NovelOutline{
		Title: "Report A",
		Chapters: []wfengine.ChapterOutline{
			{Num: 1, Title: "章1", Summary: "s1"},
			{Num: 2, Title: "章2", Summary: "s2"},
		},
	}
	if err := s.SyncOutline(ctx, wf, outline); err != nil {
		t.Fatalf("SyncOutline: %v", err)
	}
	if err := s.RecordTaskTokens(ctx, wf, "chapter-01", ai.TokenUsage{PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300}); err != nil {
		t.Fatalf("RecordTaskTokens: %v", err)
	}
	if err := s.MarkChapterDone(ctx, wf, 1, "chapters/chapter-01.md", "chapters/chapter-01.summary", 1200, nil); err != nil {
		t.Fatalf("MarkChapterDone: %v", err)
	}

	report, err := s.BuildTokenReport(ctx)
	if err != nil {
		t.Fatalf("BuildTokenReport: %v", err)
	}
	if report.NovelCount != 1 || report.TotalTokens != 300 {
		t.Fatalf("unexpected report totals: %+v", report)
	}
	if len(report.Novels) != 1 || report.Novels[0].AvgChapterTokens != 300 {
		t.Fatalf("unexpected novel row: %+v", report.Novels)
	}
	if report.DoneChapters != 1 {
		t.Fatalf("done chapters = %d, want 1", report.DoneChapters)
	}
}
