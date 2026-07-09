package workflow

import "testing"

func TestIsOutlineRAGStep(t *testing.T) {
	if !isOutlineRAGStep("outline-refine", "outline.json") {
		t.Fatal("outline-refine should refresh outline RAG")
	}
	if isOutlineRAGStep("plot-01", "chapters/chapter-01.plot.md") {
		t.Fatal("plot step should not be treated as outline refresh")
	}
}
