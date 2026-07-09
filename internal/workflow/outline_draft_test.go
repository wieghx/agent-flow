package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSnapshotOutlineDraftPreservesFirstVersion(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "draft-test", Namespace: "default"},
		Status:     agentflowiov1alpha1.WorkflowStatus{WorkspacePath: root},
	}
	draft := `{"title":"初稿","synopsis":"s","chapters":[{"num":1,"title":"章1","summary":"a"}]}`
	refined := `{"title":"精修","synopsis":"s","chapters":[{"num":1,"title":"章1","summary":"b"}]}`
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(draft), 0644); err != nil {
		t.Fatal(err)
	}
	if err := SnapshotOutlineDraft(wf); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(refined), 0644); err != nil {
		t.Fatal(err)
	}
	if err := SnapshotOutlineDraft(wf); err != nil {
		t.Fatal(err)
	}
	got, err := ReadArtifact(wf, OutlineDraftPath)
	if err != nil {
		t.Fatal(err)
	}
	if got != draft {
		t.Fatalf("draft should remain first snapshot, got %s", got)
	}
}

func TestBuildOutlineRefineMonitorContext(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "ctx-test", Namespace: "default"},
		Status:     agentflowiov1alpha1.WorkflowStatus{WorkspacePath: root},
	}
	if err := WriteArtifact(wf, OutlineDraftPath, `{"title":"初稿"}`); err != nil {
		t.Fatal(err)
	}
	ctx := BuildOutlineRefineMonitorContext(wf)
	if ctx == "" || !strings.Contains(ctx, "初稿") {
		t.Fatalf("expected draft context, got %q", ctx)
	}
}
