package rag

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertArtifactAt(t *testing.T) {
	root := t.TempDir()
	plotPath := filepath.Join(root, "chapters", "chapter-01.plot.md")
	if err := os.MkdirAll(filepath.Dir(plotPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plotPath, []byte("主角在朝堂与宰相激烈交锋，密谋逐渐浮出水面。"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := UpsertArtifactAt(root, "chapters/chapter-01.plot.md"); err != nil {
		t.Fatal(err)
	}
	idx, err := LoadIndexAt(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Chunks) != 1 || idx.Chunks[0].Source != "plot" {
		t.Fatalf("unexpected chunks: %#v", idx.Chunks)
	}

	chapterPath := filepath.Join(root, "chapters", "chapter-01.md")
	if err := os.WriteFile(chapterPath, []byte("朝堂之上，宰相拱手而立，主角冷眼旁观，气氛凝重。"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := UpsertArtifactAt(root, "chapters/chapter-01.md"); err != nil {
		t.Fatal(err)
	}
	idx, err = LoadIndexAt(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.Chunks) != 2 {
		t.Fatalf("expected 2 chunks after upsert, got %d", len(idx.Chunks))
	}
}

func TestRefreshOutlineAt(t *testing.T) {
	root := t.TempDir()
	outline := `{"title":"测试","synopsis":"简介","chapters":[{"num":1,"title":"章1","summary":"主角在朝堂与宰相交锋"}]}`
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(outline), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RefreshOutlineAt(root); err != nil {
		t.Fatal(err)
	}
	idx, err := LoadIndexAt(root)
	if err != nil || len(idx.Chunks) != 1 || idx.Chunks[0].Source != "outline" {
		t.Fatalf("unexpected outline chunks: %#v err=%v", idx, err)
	}

	outline2 := `{"title":"测试","synopsis":"简介","chapters":[{"num":1,"title":"章1","summary":"宰相倒台，主角掌权"}]}`
	if err := os.WriteFile(filepath.Join(root, "outline.json"), []byte(outline2), 0644); err != nil {
		t.Fatal(err)
	}
	plotPath := filepath.Join(root, "chapters", "chapter-01.plot.md")
	if err := os.MkdirAll(filepath.Dir(plotPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plotPath, []byte("剧情脚本内容足够长以通过最小长度校验限制。"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := UpsertArtifactAt(root, "chapters/chapter-01.plot.md"); err != nil {
		t.Fatal(err)
	}

	if err := RefreshOutlineAt(root); err != nil {
		t.Fatal(err)
	}
	idx, err = LoadIndexAt(root)
	if err != nil {
		t.Fatal(err)
	}
	var outlineCount int
	for _, ch := range idx.Chunks {
		if ch.Source == "outline" {
			outlineCount++
			if !strings.Contains(ch.Text, "宰相倒台") {
				t.Fatalf("outline not refreshed: %s", ch.Text)
			}
		}
	}
	if outlineCount != 1 {
		t.Fatalf("expected 1 outline chunk, got %d in %#v", outlineCount, idx.Chunks)
	}
}