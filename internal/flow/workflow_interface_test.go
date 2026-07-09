package flow

import (
	"strings"
	"testing"
)

func TestParseWorkflowMarker(t *testing.T) {
	resp := "好的，我来为您规划小说工作流。[CREATE_WORKFLOW:novel-outline-chapters:{\"chapterCount\":\"5\"}]"
	clean, wf := parseWorkflowMarker(resp)
	if wf == nil {
		t.Fatal("expected workflow request")
	}
	if wf.Template != "novel-team-chapters" {
		t.Fatalf("unexpected template: %s", wf.Template)
	}
	if wf.Params["teamMode"] != "true" {
		t.Fatalf("expected teamMode default on")
	}
	if wf.Params["chapterCount"] != "5" {
		t.Fatalf("unexpected chapterCount: %s", wf.Params["chapterCount"])
	}
	if strings.Contains(clean, "CREATE_WORKFLOW") {
		t.Fatalf("marker should be stripped from response: %s", clean)
	}
}

func TestParseWorkflowMarkerMergesDefaults(t *testing.T) {
	resp := "[CREATE_WORKFLOW:novel-outline-chapters:{\"chapterCount\":\"20\"}]"
	_, wf := parseWorkflowMarker(resp)
	if wf == nil {
		t.Fatal("expected workflow request")
	}
	if wf.Params["chapterCount"] != "20" {
		t.Fatalf("chapterCount: %s", wf.Params["chapterCount"])
	}
	if wf.Params["maxParallel"] != "8" {
		t.Fatalf("expected default maxParallel, got %s", wf.Params["maxParallel"])
	}
}

func TestInferNovelWorkflowRequest(t *testing.T) {
	wf := InferNovelWorkflowRequest("帮我写一本100章的荒岛生存小说")
	if wf == nil {
		t.Fatal("expected workflow request")
	}
	if wf.Params["chapterCount"] != "100" {
		t.Fatalf("chapterCount: %s", wf.Params["chapterCount"])
	}
	if wf.Prompt == "" {
		t.Fatal("expected prompt to be set")
	}
}

func TestInferNovelWorkflowRequestRejectsUnrelated(t *testing.T) {
	if InferNovelWorkflowRequest("今天天气怎么样") != nil {
		t.Fatal("expected nil for unrelated message")
	}
}

func TestNewWorkflowCRD(t *testing.T) {
	wf, err := NewWorkflowCRD("novel-team-chapters", "荒岛生存小说", map[string]string{"chapterCount": "3"}, "wf-demo", "default")
	if err != nil {
		t.Fatalf("NewWorkflowCRD failed: %v", err)
	}
	if wf.Name != "wf-demo" {
		t.Fatalf("unexpected name: %s", wf.Name)
	}
	if wf.Spec.Template != "novel-team-chapters" {
		t.Fatalf("unexpected spec template: %s", wf.Spec.Template)
	}
	if len(wf.Spec.Steps) != 6 {
		t.Fatalf("expected 6 steps (outline+refine+bible+plots+chapters+merge), got %d", len(wf.Spec.Steps))
	}
	if wf.Labels["agentflow.io/source"] != "chat" {
		t.Fatalf("expected chat source label")
	}
}
