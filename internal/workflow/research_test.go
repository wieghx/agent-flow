package workflow

import "testing"

func TestHistoricalResearchEnabled(t *testing.T) {
	if !HistoricalResearchEnabled(map[string]string{"historicalResearch": "true"}, "") {
		t.Fatal("explicit param should enable research")
	}
	if !HistoricalResearchEnabled(nil, "写一部唐朝长安的历史小说") {
		t.Fatal("historical intent should enable research")
	}
	if HistoricalResearchEnabled(nil, "都市悬疑短篇") {
		t.Fatal("non-historical should not enable research")
	}
}

func TestNovelProductionSpecHistoricalStep(t *testing.T) {
	spec := novelProductionSpec("写一部明朝锦衣卫题材历史小说", map[string]string{
		"chapterCount": "3",
		"teamMode":     "true",
	}, "novel-team-historical")
	if spec.Template != "novel-team-chapters" {
		t.Fatalf("template=%s", spec.Template)
	}
	foundResearch, foundOutline := false, false
	for _, step := range spec.Steps {
		if step.ID == "historical-research" {
			foundResearch = true
			if !step.TaskSpec.MCPMode {
				t.Fatal("research step needs MCP")
			}
		}
		if step.ID == "outline" {
			foundOutline = true
			for _, dep := range step.DependsOn {
				if dep == "historical-research" {
					foundResearch = true
				}
			}
		}
	}
	if !foundResearch || !foundOutline {
		t.Fatalf("steps missing research=%v outline=%v", foundResearch, foundOutline)
	}
}