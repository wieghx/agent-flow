package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExpandChaptersInsertsArcSteps(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "arc-test", Namespace: "default"},
		Status:     agentflowiov1alpha1.WorkflowStatus{WorkspacePath: root},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Params: map[string]string{"chapterCount": "12", "arcInterval": "10"},
			Steps: []agentflowiov1alpha1.WorkflowStep{
				{
					ID:   "chapters",
					Type: agentflowiov1alpha1.WorkflowStepTypeForeach,
					Foreach: &agentflowiov1alpha1.WorkflowForeach{
						Source:       "outline.json",
						StepIDPrefix: "chapter",
						OutputPath:   "chapters/chapter-{{num}}.md",
						Instruction:  "写章节",
					},
				},
			},
		},
	}

	var chapters []string
	for i := 1; i <= 12; i++ {
		chapters = append(chapters, fmt.Sprintf("{\"num\":%d,\"title\":\"章\",\"summary\":\"s\"}", i))
	}
	outline := "{\"title\":\"书\",\"synopsis\":\"介\",\"chapters\":[" + joinComma(chapters) + "]}"
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(outline), 0644); err != nil {
		t.Fatal(err)
	}

	steps, err := ResolveSteps(wf)
	if err != nil {
		t.Fatalf("ResolveSteps failed: %v", err)
	}
	if len(steps) != 13 {
		t.Fatalf("expected 12 chapters + 1 arc, got %d", len(steps))
	}
	if steps[10].ID != "arc-10" {
		t.Fatalf("expected arc-10 after chapter 10, got %s", steps[10].ID)
	}
}

func TestChapterDependsOnArc(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Params: map[string]string{"arcInterval": "10"},
		},
	}
	completed := map[string]bool{
		"chapter-10": true,
	}
	if dependenciesMet(wf, "chapter-11", map[string][]string{"chapter": {"outline"}}, completed) {
		t.Fatal("chapter 11 should wait for arc-10")
	}
	completed["arc-10"] = true
	if !dependenciesMet(wf, "chapter-11", map[string][]string{"chapter": {"outline"}}, completed) {
		t.Fatal("chapter 11 should proceed after arc-10")
	}
}

func TestLoadArcSummaries(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		Status: agentflowiov1alpha1.WorkflowStatus{WorkspacePath: root},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Params: map[string]string{"arcInterval": "10"},
		},
	}
	if err := os.MkdirAll(filepath.Join(root, "arcs"), 0755); err != nil {
		t.Fatal(err)
	}
	content := "本弧主角完成第一次团队整合。"
	if err := os.WriteFile(filepath.Join(root, "arcs/arc-01-10.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	got := LoadArcSummaries(wf, 15, 2)
	if got == "" || !containsSubstring(got, "团队整合") {
		t.Fatalf("expected arc summary in context, got %q", got)
	}
}

func joinComma(items []string) string {
	if len(items) == 0 {
		return ""
	}
	out := items[0]
	for i := 1; i < len(items); i++ {
		out += "," + items[i]
	}
	return out
}
