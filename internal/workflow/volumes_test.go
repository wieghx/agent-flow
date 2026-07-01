package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPlanVolumes(t *testing.T) {
	vols := PlanVolumes(100, 25)
	if len(vols) != 4 {
		t.Fatalf("expected 4 volumes, got %d", len(vols))
	}
	if vols[3].End != 100 {
		t.Fatalf("last volume should end at 100, got %d", vols[3].End)
	}
}

func TestUseVolumeOutline(t *testing.T) {
	if !UseVolumeOutline(map[string]string{}, 100) {
		t.Fatal("100 chapters should use volume outline")
	}
	if UseVolumeOutline(map[string]string{}, 10) {
		t.Fatal("10 chapters should use simple outline")
	}
}

func TestMergeVolumeOutlines(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "vol-merge", Namespace: "default"},
		Status:     agentflowiov1alpha1.WorkflowStatus{WorkspacePath: root},
	}
	skeleton := `{"title":"测试书","synopsis":"简介","characters":[],"volumes":[{"num":1,"title":"卷1","startChapter":1,"endChapter":2,"theme":"t","summary":"s"}]}`
	vol1 := `{"volume":1,"chapters":[{"num":1,"title":"章1","summary":"s1"},{"num":2,"title":"章2","summary":"s2"}]}`
	if err := os.WriteFile(filepath.Join(root, "skeleton.json"), []byte(skeleton), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "volumes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "volumes/volume-01.json"), []byte(vol1), 0644); err != nil {
		t.Fatal(err)
	}

	merged, err := MergeVolumeOutlines(wf)
	if err != nil {
		t.Fatalf("MergeVolumeOutlines failed: %v", err)
	}
	outline, err := ParseOutlineJSON(merged)
	if err != nil {
		t.Fatalf("ParseOutlineJSON failed: %v", err)
	}
	if len(outline.Chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(outline.Chapters))
	}
}

func TestMergeVolumeOutlinesRejectsTruncatedVolume(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "vol-invalid", Namespace: "default"},
		Status:     agentflowiov1alpha1.WorkflowStatus{WorkspacePath: root},
	}
	skeleton := `{"title":"测试书","synopsis":"简介","characters":[],"volumes":[{"num":1,"title":"卷1","startChapter":76,"endChapter":100,"theme":"t","summary":"s"}]}`
	validVol := `{"volume":1,"chapters":[`
	for n := 76; n <= 100; n++ {
		if n > 76 {
			validVol += ","
		}
		validVol += fmt.Sprintf(`{"num":%d,"title":"章%d","summary":"梗概"}`, n, n)
	}
	validVol += `]}`

	truncated := `{"volume":1,"chapters":[{"num":76,"title":"章76","summary":"梗概"},{"`
	if err := os.WriteFile(filepath.Join(root, "skeleton.json"), []byte(skeleton), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "volumes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "volumes/volume-01.json"), []byte(truncated), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := MergeVolumeOutlines(wf); err == nil {
		t.Fatal("expected merge to fail on truncated volume JSON")
	}

	if err := os.WriteFile(filepath.Join(root, "volumes/volume-01.json"), []byte(validVol), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := MergeVolumeOutlines(wf); err != nil {
		t.Fatalf("expected merge to succeed with complete volume JSON: %v", err)
	}
}

func TestVolumeOutlineSpecSteps(t *testing.T) {
	spec := novelProductionSpec("prompt", map[string]string{"chapterCount": "100", "teamMode": "false"}, "novel-outline-chapters")
	if spec.Steps[0].ID != "outline-skeleton" {
		t.Fatalf("expected skeleton step first, got %s", spec.Steps[0].ID)
	}
	if spec.Steps[0].TaskSpec.MonitorTaskType != "novel-outline-skeleton" {
		t.Fatalf("skeleton monitor type = %q", spec.Steps[0].TaskSpec.MonitorTaskType)
	}
	foundMerge := false
	foundRefine := false
	chaptersDep := ""
	for _, step := range spec.Steps {
		if step.ID == "outline-merge" {
			foundMerge = true
		}
		if step.ID == "outline-refine" {
			foundRefine = true
			if len(step.DependsOn) != 1 || step.DependsOn[0] != "outline-merge" {
				t.Fatalf("outline-refine should depend on outline-merge, got %#v", step.DependsOn)
			}
		}
		if step.ID == "chapters" {
			chaptersDep = step.DependsOn[0]
		}
	}
	if !foundMerge {
		t.Fatal("expected outline-merge step")
	}
	if !foundRefine {
		t.Fatal("expected outline-refine step")
	}
	if chaptersDep != "outline-refine" {
		t.Fatalf("chapters should depend on outline-refine, got %q", chaptersDep)
	}
	for _, step := range spec.Steps {
		if step.ID == "outline-vol-01" {
			if step.TaskSpec.MonitorTaskType != "novel-volume-outline" {
				t.Fatalf("volume step monitor type = %q, want novel-volume-outline", step.TaskSpec.MonitorTaskType)
			}
			return
		}
	}
	t.Fatal("expected outline-vol-01 step")
}
