package rag

import "testing"

func TestCosineSimilarity(t *testing.T) {
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	if cosineSimilarity(a, b) < 0.99 {
		t.Fatal("identical vectors should score ~1")
	}
	c := []float32{0, 1, 0}
	if cosineSimilarity(a, c) > 0.01 {
		t.Fatal("orthogonal vectors should score ~0")
	}
}

func TestBM25Normalization(t *testing.T) {
	chunks := []Chunk{
		{ID: "a", Text: "朝堂 宰相 密谋"},
		{ID: "b", Text: "江湖 侠客 行走"},
	}
	scores := bm25Scores(chunks, "朝堂 宰相")
	if scores["a"] <= scores["b"] {
		t.Fatalf("expected higher score for matching chunk: %#v", scores)
	}
	if scores["a"] > 1.01 || scores["a"] < 0 {
		t.Fatalf("normalized score out of range: %f", scores["a"])
	}
}
