package rag

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearchRAG(t *testing.T) {
	root := t.TempDir()
	outline := `{"title":"测试","synopsis":"简介","chapters":[{"num":1,"title":"章1","summary":"主角在朝堂与宰相交锋"}]}`
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(outline), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := RebuildIndexAt(root); err != nil {
		t.Fatal(err)
	}
	chunks, err := SearchAt(root, "朝堂 宰相", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected rag hits")
	}
}
