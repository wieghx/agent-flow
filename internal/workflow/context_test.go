package workflow

import (
	"os"
	"path/filepath"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildChapterContextRollingWindow(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "ctx-test", Namespace: "default"},
		Status: agentflowiov1alpha1.WorkflowStatus{
			WorkspacePath: root,
		},
	}
	chaptersDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chaptersDir, 0755); err != nil {
		t.Fatal(err)
	}

	for num := 1; num <= 6; num++ {
		file := ChapterFileName(num, 2)
		content := "章节正文" + string(rune('0'+num)) + "。" + "结尾片段" + string(rune('0'+num))
		if err := os.WriteFile(filepath.Join(chaptersDir, file), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		summary := ChapterSummaryFileName(num, 2)
		if err := os.WriteFile(filepath.Join(chaptersDir, summary), []byte("摘要"+string(rune('0'+num))), 0644); err != nil {
			t.Fatal(err)
		}
	}

	ctx := BuildChapterContext(wf, 7, 2, 3)
	if ctx.PreviousEnding == "" {
		t.Fatal("expected previous chapter ending")
	}
	if ctx.RecentSummaries == "" {
		t.Fatal("expected recent summaries")
	}
	if ctx.StorySoFar == "" {
		t.Fatal("expected compressed story-so-far")
	}
}
