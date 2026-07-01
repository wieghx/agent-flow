package store

import (
	"context"
	"database/sql"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	_ "modernc.org/sqlite"
	wfengine "agent-flow/internal/workflow"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLibraryListAndDelete(t *testing.T) {
	db, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()
	s := &SQLStore{db: db, driver: "sqlite"}

	ctx := context.Background()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "lib-demo", Namespace: "default"},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Params: map[string]string{"chapterCount": "3"},
		},
	}
	if err := s.EnsureNovel(ctx, wf); err != nil {
		t.Fatalf("EnsureNovel: %v", err)
	}
	outline := &wfengine.NovelOutline{
		Title:    "测试书名",
		Synopsis: "测试简介",
		Chapters: []wfengine.ChapterOutline{
			{Num: 1, Title: "第一章", Summary: "开端"},
			{Num: 2, Title: "第二章", Summary: "发展"},
			{Num: 3, Title: "第三章", Summary: "结局"},
		},
	}
	if err := s.SyncOutline(ctx, wf, outline); err != nil {
		t.Fatalf("SyncOutline: %v", err)
	}
	if err := s.MarkChapterDone(ctx, wf, 1, "chapters/chapter-001.md", "chapters/chapter-001.summary", 1200, nil); err != nil {
		t.Fatalf("MarkChapterDone: %v", err)
	}

	entries, err := s.ListLibrary(ctx)
	if err != nil {
		t.Fatalf("ListLibrary: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected library entries")
	}
	found := false
	for _, e := range entries {
		if e.Name == "lib-demo" {
			found = true
			if e.Title != "测试书名" {
				t.Fatalf("title: %s", e.Title)
			}
			if e.DoneChapters != 1 {
				t.Fatalf("done chapters: %d", e.DoneChapters)
			}
		}
	}
	if !found {
		t.Fatal("lib-demo not found")
	}

	if err := s.DeleteNovelRecord(ctx, "default", "lib-demo"); err != nil {
		t.Fatalf("DeleteNovelRecord: %v", err)
	}
	entries, _ = s.ListLibrary(ctx)
	for _, e := range entries {
		if e.Name == "lib-demo" {
			t.Fatal("record should be deleted")
		}
	}
}

func openTestDB(t *testing.T) (*sql.DB, error) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	if err := migrate(db, "sqlite"); err != nil {
		return nil, err
	}
	return db, nil
}