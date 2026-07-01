package workflow

import (
	"os"
	"path/filepath"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildConsistencyMonitorContext(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "novel-wf", Namespace: "default"},
		Status:     agentflowiov1alpha1.WorkflowStatus{WorkspacePath: root},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Params: map[string]string{"wordsPerChapter": "2500", "contextChapters": "3"},
		},
	}

	chaptersDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chaptersDir, 0755); err != nil {
		t.Fatal(err)
	}
	outline := `{"title":"荒岛书","synopsis":"简介","characters":[{"name":"林峰","role":"主角","trait":"冷静"}],"chapters":[{"num":1,"title":"风暴","summary":"遭遇风暴"},{"num":2,"title":"登岛","summary":"登上荒岛"}]}`
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(outline), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chaptersDir, "chapter-01.md"), []byte("林峰在甲板上眺望大海。风暴来临。"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chaptersDir, "chapter-01.summary"), []byte("林峰遭遇风暴"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := BuildConsistencyMonitorContext(wf, "chapter-02")
	if ctx == "" {
		t.Fatal("expected consistency context")
	}
	for _, want := range []string{"林峰", "上一章结尾", "本章大纲要求", "登岛"} {
		if !containsSubstring(ctx, want) {
			t.Fatalf("context missing %q: %s", want, ctx)
		}
	}
}

func TestMinChapterLength(t *testing.T) {
	if MinChapterLength(2500) != 500 {
		t.Fatalf("unexpected min length: %d", MinChapterLength(2500))
	}
}
