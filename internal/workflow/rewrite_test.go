package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRewriteOutputPath(t *testing.T) {
	params := map[string]string{
		ParamRewriteChapter: "3",
		ParamRewriteLayer:   RewriteLayerPlot,
		"chapterCount":      "10",
	}
	if got := RewriteOutputPath(params); got != "chapters/chapter-03.plot.md" {
		t.Fatalf("plot path = %q", got)
	}
	params[ParamRewriteLayer] = RewriteLayerChapter
	if got := RewriteOutputPath(params); got != "chapters/chapter-03.md" {
		t.Fatalf("chapter path = %q", got)
	}
}

func TestWorkspacePathShared(t *testing.T) {
	shared := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "child", Namespace: "default"},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Params: map[string]string{ParamSharedWorkspace: shared},
		},
	}
	if got := WorkspacePath(wf); got != shared {
		t.Fatalf("shared workspace = %q want %q", got, shared)
	}
}

func TestBuildRewriteInstruction(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Params: map[string]string{
				ParamRewriteChapter: "1",
				ParamRewriteLayer:   RewriteLayerChapter,
				ParamRewriteNote:    "加强开篇悬念",
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{WorkspacePath: root},
	}
	outline := `{"title":"测试书","synopsis":"简介","chapters":[{"num":1,"title":"风起","summary":"主角入京"}]}`
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(outline), 0644); err != nil {
		t.Fatal(err)
	}
	chPath := filepath.Join(root, "chapters", "chapter-01.md")
	if err := os.MkdirAll(filepath.Dir(chPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chPath, []byte("这是已有的第一章正文，需要在此基础上修改。"), 0644); err != nil {
		t.Fatal(err)
	}
	instr, err := BuildRewriteInstruction(wf)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"加强开篇悬念", "当前正文", "测试书"} {
		if !strings.Contains(instr, want) {
			t.Fatalf("instruction missing %q: %s", want, instr)
		}
	}
}
