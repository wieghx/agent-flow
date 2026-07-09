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

func TestRecordTaskTokens(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tokens.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = st.Close() }()
	s, ok := st.(*SQLStore)
	if !ok {
		t.Fatal("expected *SQLStore")
	}

	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "novel-tokens", Namespace: "default"},
	}
	ctx := context.Background()
	outline := &wfengine.NovelOutline{
		Title: "Token Test",
		Chapters: []wfengine.ChapterOutline{
			{Num: 1, Title: "章1", Summary: "s1"},
		},
	}
	if err := s.SyncOutline(ctx, wf, outline); err != nil {
		t.Fatalf("SyncOutline: %v", err)
	}

	usage := ai.TokenUsage{PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300}
	if err := s.RecordTaskTokens(ctx, wf, "plot-01", usage); err != nil {
		t.Fatalf("RecordTaskTokens plot: %v", err)
	}
	if err := s.RecordTaskTokens(ctx, wf, "chapter-01", usage); err != nil {
		t.Fatalf("RecordTaskTokens chapter: %v", err)
	}

	lib, err := s.GetLibrary(ctx, "default", "novel-tokens")
	if err != nil || lib == nil {
		t.Fatalf("GetLibrary: %v", err)
	}
	if lib.TotalTokens != 600 {
		t.Fatalf("novel total tokens = %d, want 600", lib.TotalTokens)
	}

	chapters, err := s.ListChapters(ctx, "default", "novel-tokens")
	if err != nil {
		t.Fatalf("ListChapters: %v", err)
	}
	if len(chapters) != 1 || chapters[0].TotalTokens != 600 {
		t.Fatalf("chapter tokens = %+v, want 600 on chapter 1", chapters)
	}
}
