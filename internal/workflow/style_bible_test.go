package workflow

import "testing"

func TestParseStyleBibleJSON(t *testing.T) {
	raw := `{"title":"测试","pov":"第三人称","protagonists":["苏晴"],"forbidden":["换主角"]}`
	b, err := ParseStyleBibleJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if b.Protagonists[0] != "苏晴" {
		t.Fatalf("protagonist = %v", b.Protagonists)
	}
}

func TestTeamTemplateHasStyleBibleStep(t *testing.T) {
	spec := novelProductionSpec("prompt", DefaultNovelParams(5), "novel-team-chapters")
	foundOutline, foundRefine, foundBible, foundChapters := false, false, false, false
	for _, step := range spec.Steps {
		switch step.ID {
		case "outline":
			foundOutline = true
		case "outline-refine":
			foundRefine = true
			if len(step.DependsOn) != 1 || step.DependsOn[0] != "outline" {
				t.Fatalf("outline-refine should depend on outline, got %#v", step.DependsOn)
			}
		case "style-bible":
			foundBible = true
			if len(step.DependsOn) != 1 || step.DependsOn[0] != "outline-refine" {
				t.Fatalf("style-bible should depend on outline-refine, got %#v", step.DependsOn)
			}
		case "chapters":
			foundChapters = true
			if len(step.DependsOn) < 2 {
				t.Fatalf("chapters should depend on outline-refine+bible: %v", step.DependsOn)
			}
		}
	}
	if !foundOutline || !foundRefine || !foundBible || !foundChapters {
		t.Fatalf("steps missing: outline=%v refine=%v bible=%v chapters=%v", foundOutline, foundRefine, foundBible, foundChapters)
	}
	if spec.Template != "novel-team-chapters" {
		t.Fatalf("template = %s", spec.Template)
	}
}