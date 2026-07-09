package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResolveStepsRequiresOutlineArtifact(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "test-wf", Namespace: "default"},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Template: "novel-outline-chapters",
			Prompt:   "写一部生存小说",
			Params:   map[string]string{"chapterCount": "2", "teamMode": "false"},
		},
	}
	steps, err := ResolveSteps(wf)
	if err != nil {
		t.Fatalf("ResolveSteps failed before outline: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("expected outline+refine+merge before foreach expand, got %d: %#v", len(steps), steps)
	}
}

func TestResolveStepsExpandsChaptersAfterOutline(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "test-wf", Namespace: "default"},
		Status: agentflowiov1alpha1.WorkflowStatus{
			WorkspacePath: root,
		},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Template: "novel-outline-chapters",
			Prompt:   "写一部生存小说",
			Params:   map[string]string{"chapterCount": "2", "teamMode": "false"},
		},
	}
	outline := `{"title":"测试书","synopsis":"简介","chapters":[{"num":1,"title":"章1","summary":"s1"},{"num":2,"title":"章2","summary":"s2"}]}`
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(outline), 0644); err != nil {
		t.Fatal(err)
	}

	steps, err := ResolveSteps(wf)
	if err != nil {
		t.Fatalf("ResolveSteps failed: %v", err)
	}
	if len(steps) != 7 {
		t.Fatalf("expected outline+refine+2 plots+2 chapters+merge, got %d steps", len(steps))
	}
	if steps[0].ID != "outline" || steps[1].ID != "outline-refine" {
		t.Fatalf("unexpected prefix: %s %s", steps[0].ID, steps[1].ID)
	}
	if steps[2].ID != "plot-01" || steps[4].ID != "chapter-01" || steps[6].ID != "merge" {
		t.Fatalf("unexpected expanded ids: %#v", []string{steps[2].ID, steps[4].ID, steps[6].ID})
	}
}

func TestNextStepAfterSkeleton100Ch(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "novel-survival-100", Namespace: "default"},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Template: "novel-outline-chapters",
			Prompt:   "长篇荒岛生存小说",
			Params: map[string]string{
				"chapterCount":    "100",
				"volumeSize":      "25",
				"arcInterval":     "10",
				"wordsPerChapter": "2500",
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CompletedSteps: []string{"outline-skeleton"},
			StepStatuses: []agentflowiov1alpha1.WorkflowStepStatus{
				{ID: "outline-skeleton", Phase: agentflowiov1alpha1.TaskPhaseSucceeded},
			},
		},
	}
	resolved, err := ResolveSteps(wf)
	if err != nil {
		t.Fatalf("ResolveSteps failed: %v", err)
	}
	if len(resolved) < 6 {
		t.Fatalf("expected volume outline steps, got %d: %#v", len(resolved), stepIDs(resolved))
	}
	next, err := NextStep(wf, resolved)
	if err != nil {
		t.Fatalf("NextStep failed: %v", err)
	}
	if next == nil || next.ID != "outline-vol-01" {
		t.Fatalf("expected outline-vol-01, got %#v", next)
	}
}

func stepIDs(steps []ResolvedStep) []string {
	ids := make([]string, len(steps))
	for i, s := range steps {
		ids[i] = s.ID
	}
	return ids
}

func TestNextStepChapter002AfterChapter001Padded100(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "novel-survival-100", Namespace: "default"},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Template: "novel-outline-chapters",
			Params: map[string]string{
				"chapterCount": "100",
				"volumeSize":   "25",
				"teamMode":     "false",
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			WorkspacePath: root,
			CompletedSteps: []string{
				"outline-skeleton", "outline-vol-01", "outline-vol-02",
				"outline-vol-03", "outline-vol-04", "outline-merge", "outline-refine",
				"plot-001", "chapter-001",
			},
		},
	}
	var chapters strings.Builder
	chapters.WriteString(`{"title":"测试书","synopsis":"简介","chapters":[`)
	for i := 1; i <= 100; i++ {
		if i > 1 {
			chapters.WriteByte(',')
		}
		fmt.Fprintf(&chapters, `{"num":%d,"title":"章%d","summary":"s%d"}`, i, i, i)
	}
	chapters.WriteString(`]}`)
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(chapters.String()), 0644); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveSteps(wf)
	if err != nil {
		t.Fatalf("ResolveSteps failed: %v", err)
	}
	next, err := NextStep(wf, resolved)
	if err != nil {
		t.Fatalf("NextStep failed: %v", err)
	}
	if next == nil || next.ID != "plot-002" {
		t.Fatalf("expected plot-002 after chapter-001 with three-stage pipeline, got %#v", next)
	}
}

func TestReadyStepsParallelVolumesAfterSkeleton100Ch(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "novel-survival-100", Namespace: "default"},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Template: "novel-outline-chapters",
			Params: map[string]string{
				"chapterCount": "100",
				"volumeSize":   "25",
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CompletedSteps: []string{"outline-skeleton"},
		},
	}
	resolved, err := ResolveSteps(wf)
	if err != nil {
		t.Fatalf("ResolveSteps failed: %v", err)
	}
	ready, err := ReadySteps(wf, resolved)
	if err != nil {
		t.Fatalf("ReadySteps failed: %v", err)
	}
	if len(ready) != 4 {
		t.Fatalf("expected 4 parallel volume steps, got %d: %#v", len(ready), stepIDs(ready))
	}
	for i, id := range []string{"outline-vol-01", "outline-vol-02", "outline-vol-03", "outline-vol-04"} {
		if ready[i].ID != id {
			t.Fatalf("ready[%d] = %s, want %s", i, ready[i].ID, id)
		}
	}
}

func TestReadyStepsOutlineMergeWaitsForAllVolumes(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Template: "novel-outline-chapters",
			Params: map[string]string{
				"chapterCount": "100",
				"volumeSize":   "25",
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CompletedSteps: []string{
				"outline-skeleton", "outline-vol-01", "outline-vol-02", "outline-vol-03",
			},
		},
	}
	resolved, err := ResolveSteps(wf)
	if err != nil {
		t.Fatalf("ResolveSteps failed: %v", err)
	}
	ready, err := ReadySteps(wf, resolved)
	if err != nil {
		t.Fatalf("ReadySteps failed: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != "outline-vol-04" {
		t.Fatalf("expected only outline-vol-04, got %#v", stepIDs(ready))
	}
}

func TestReadyStepsPipelineChaptersBatch(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "pipeline-test", Namespace: "default"},
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Template: "novel-outline-chapters",
			Params:   map[string]string{"chapterCount": "8", "teamMode": "false"},
			Execution: agentflowiov1alpha1.WorkflowExecution{
				Mode:            ExecutionModeParallel,
				MaxParallel:     4,
				ChapterMode:     ChapterModePipeline,
				ChapterPipeline: 4,
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			WorkspacePath:  root,
			CompletedSteps: []string{"outline", "outline-refine", "plot-01", "plot-02", "plot-03", "plot-04"},
		},
	}
	var ob strings.Builder
	ob.WriteString(`{"title":"测试书","synopsis":"简介","chapters":[`)
	for i := 1; i <= 8; i++ {
		if i > 1 {
			ob.WriteByte(',')
		}
		fmt.Fprintf(&ob, `{"num":%d,"title":"章%d","summary":"s%d"}`, i, i, i)
	}
	ob.WriteString(`]}`)
	outline := ob.String()
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(outline), 0644); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveSteps(wf)
	if err != nil {
		t.Fatalf("ResolveSteps failed: %v", err)
	}
	ready, err := ReadySteps(wf, resolved)
	if err != nil {
		t.Fatalf("ReadySteps failed: %v", err)
	}
	if len(ready) != 8 {
		t.Fatalf("expected 4 chapters + 4 plots in pipeline batch, got %d: %#v", len(ready), stepIDs(ready))
	}
	for i, id := range []string{"chapter-01", "chapter-02", "chapter-03", "chapter-04"} {
		if ready[i].ID != id {
			t.Fatalf("ready[%d] = %s, want %s", i, ready[i].ID, id)
		}
	}
	for i, id := range []string{"plot-05", "plot-06", "plot-07", "plot-08"} {
		if ready[i+4].ID != id {
			t.Fatalf("ready[%d] = %s, want %s", i+4, ready[i+4].ID, id)
		}
	}

	wf.Status.CompletedSteps = append(wf.Status.CompletedSteps,
		"chapter-01", "chapter-02", "chapter-03", "chapter-04",
		"plot-05", "plot-06", "plot-07", "plot-08",
	)
	ready, err = ReadySteps(wf, resolved)
	if err != nil {
		t.Fatalf("ReadySteps batch2 failed: %v", err)
	}
	if len(ready) != 4 || ready[0].ID != "chapter-05" {
		t.Fatalf("expected chapter-05..08 batch, got %#v", stepIDs(ready))
	}
}

func TestChapterGateNumPipeline(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Execution: agentflowiov1alpha1.WorkflowExecution{
				ChapterMode:     ChapterModePipeline,
				ChapterPipeline: 4,
			},
		},
	}
	if got := chapterGateNum(wf, 3); got != 0 {
		t.Fatalf("chapter 3 gate = %d, want 0", got)
	}
	if got := chapterGateNum(wf, 5); got != 1 {
		t.Fatalf("chapter 5 gate = %d, want 1", got)
	}
	wf.Spec.Execution.ChapterMode = ChapterModeSequential
	if got := chapterGateNum(wf, 2); got != 1 {
		t.Fatalf("sequential chapter 2 gate = %d, want 1", got)
	}
	if got := chapterGateNum(wf, 5); got != 4 {
		t.Fatalf("sequential chapter 5 gate = %d, want 4", got)
	}
}

func TestStepAutoRetryDefaults(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{}
	if got := StepMaxRetries(wf); got != DefaultStepMaxRetries {
		t.Fatalf("StepMaxRetries() = %d, want %d", got, DefaultStepMaxRetries)
	}
	if !StepAutoRetryEnabled(wf) {
		t.Fatal("expected auto-retry enabled by default")
	}
	wf.Spec.Execution.StepMaxRetries = 2
	if got := StepMaxRetries(wf); got != 2 {
		t.Fatalf("StepMaxRetries() = %d, want 2", got)
	}
}

func TestReadyStepsAfterRetryClearsFailedPhase(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Steps: []agentflowiov1alpha1.WorkflowStep{
				{ID: "outline", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
				{ID: "chapter-01", Type: agentflowiov1alpha1.WorkflowStepTypeAITask, DependsOn: []string{"outline"}},
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CompletedSteps: []string{"outline"},
			StepStatuses: []agentflowiov1alpha1.WorkflowStepStatus{
				{ID: "chapter-01", Retries: 1, Message: "auto-retry 1/5"},
			},
		},
	}
	resolved := []ResolvedStep{
		{ID: "outline", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
		{ID: "chapter-01", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
	}
	ready, err := ReadySteps(wf, resolved)
	if err != nil {
		t.Fatalf("ReadySteps failed: %v", err)
	}
	if len(ready) != 1 || ready[0].ID != "chapter-01" {
		t.Fatalf("expected retriable chapter-01 ready, got %#v", ready)
	}
}

func TestMaxParallelDefault(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{}
	if got := MaxParallel(wf); got != DefaultMaxParallel {
		t.Fatalf("MaxParallel() = %d, want %d", got, DefaultMaxParallel)
	}
	wf.Spec.Execution.MaxParallel = 8
	if got := MaxParallel(wf); got != 8 {
		t.Fatalf("MaxParallel() = %d, want 8", got)
	}
}

func TestNextStepRespectsDependencies(t *testing.T) {
	wf := &agentflowiov1alpha1.Workflow{
		Spec: agentflowiov1alpha1.WorkflowSpec{
			Steps: []agentflowiov1alpha1.WorkflowStep{
				{ID: "outline", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
				{ID: "merge", Type: agentflowiov1alpha1.WorkflowStepTypeMerge, DependsOn: []string{"chapters"}},
			},
		},
		Status: agentflowiov1alpha1.WorkflowStatus{
			CompletedSteps: []string{"outline"},
		},
	}
	resolved := []ResolvedStep{
		{ID: "outline", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
		{ID: "chapter-01", Type: agentflowiov1alpha1.WorkflowStepTypeAITask},
		{ID: "merge", Type: agentflowiov1alpha1.WorkflowStepTypeMerge},
	}
	next, err := NextStep(wf, resolved)
	if err != nil {
		t.Fatalf("NextStep failed: %v", err)
	}
	if next == nil || next.ID != "chapter-01" {
		t.Fatalf("expected chapter-01 as next step, got %#v", next)
	}
}

func TestMergeChapterFiles(t *testing.T) {
	root := t.TempDir()
	wf := &agentflowiov1alpha1.Workflow{
		ObjectMeta: metav1.ObjectMeta{Name: "merge-test", Namespace: "default"},
		Status: agentflowiov1alpha1.WorkflowStatus{
			WorkspacePath: root,
		},
	}
	chaptersDir := filepath.Join(root, "chapters")
	if err := os.MkdirAll(chaptersDir, 0755); err != nil {
		t.Fatal(err)
	}
	outline := `{"title":"测试书","synopsis":"简介","chapters":[{"num":1,"title":"第一章","summary":"开始"}]}`
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(outline), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chaptersDir, "chapter-01.md"), []byte("# 第一章\n正文"), 0644); err != nil {
		t.Fatal(err)
	}

	book, err := MergeChapterFiles(wf)
	if err != nil {
		t.Fatalf("MergeChapterFiles failed: %v", err)
	}
	if !containsAll(book, "测试书", "第一章", "正文") {
		t.Fatalf("unexpected merged book: %s", book)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !containsSubstring(s, p) {
			return false
		}
	}
	return true
}

func containsSubstring(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
