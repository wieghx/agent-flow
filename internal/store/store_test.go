package store

import (
	"context"
	"path/filepath"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	wfengine "agent-flow/internal/workflow"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testWorkflow(t *testing.T) *agentflowiov1alpha1.Workflow {
	t.Helper()
	return &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "novel-demo", Namespace: "default"},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Params: map[string]string{"chapterCount": "4"},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			WorkspacePath: t.TempDir(),
		},
	}
}

func TestSQLStoreOutlineAndChapters(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open() err = %v", err)
	}
	defer func() { _ = s.Close() }()

	wf := testWorkflow(t)
	ctx := context.Background()
	if err := s.EnsureNovel(ctx, wf); err != nil {
		t.Fatalf("EnsureNovel() err = %v", err)
	}

	outline := &wfengine.NovelOutline{
		Title:    "测试书",
		Synopsis: "简介",
		Characters: []map[string]any{
			{"name": "林峰", "role": "主角", "trait": "冷静"},
		},
		Chapters: []wfengine.ChapterOutline{
			{Num: 1, Title: "章1", Summary: "s1"},
			{Num: 2, Title: "章2", Summary: "s2"},
			{Num: 3, Title: "章3", Summary: "s3"},
			{Num: 4, Title: "章4", Summary: "s4"},
		},
	}
	if err := s.SyncOutline(ctx, wf, outline); err != nil {
		t.Fatalf("SyncOutline() err = %v", err)
	}

	missing, err := s.MissingChapterNumbers(ctx, wf)
	if err != nil || len(missing) != 4 {
		t.Fatalf("MissingChapterNumbers() = %#v err=%v", missing, err)
	}

	if err := s.MarkChapterDone(ctx, wf, 1, "chapters/chapter-01.md", "chapters/chapter-01.summary", 1200, nil); err != nil {
		t.Fatalf("MarkChapterDone() err = %v", err)
	}
	if err := s.MarkChapterFailed(ctx, wf, 4, "chapter-04"); err != nil {
		t.Fatalf("MarkChapterFailed() err = %v", err)
	}

	missing, err = s.MissingChapterNumbers(ctx, wf)
	if err != nil {
		t.Fatalf("MissingChapterNumbers() err = %v", err)
	}
	if len(missing) != 3 || missing[0] != 2 || missing[1] != 3 || missing[2] != 4 {
		t.Fatalf("unexpected missing after updates: %#v", missing)
	}
}

func TestNoopStoreDisabled(t *testing.T) {
	s := Noop()
	if s.Enabled() {
		t.Fatal("noop store should be disabled")
	}
}
