package rag

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const IndexPath = "rag/index.json"

// Chunk is one searchable text segment.
type Chunk struct {
	ID      string `json:"id"`
	Chapter int    `json:"chapter,omitempty"`
	Title   string `json:"title,omitempty"`
	Text    string `json:"text"`
	Source  string `json:"source"`
}

// Index is the workspace RAG store.
type Index struct {
	Chunks []Chunk `json:"chunks"`
}

// RAGEnabled reports whether retrieval is on for this workflow.
func RAGEnabled(params map[string]string) bool {
	return boolParam(params, "ragEnabled", true)
}

// TopK returns retrieval count.
func TopK(params map[string]string) int {
	if v := intParam(params, "ragTopK", 0); v > 0 {
		return v
	}
	return 5
}

func boolParam(params map[string]string, key string, fallback bool) bool {
	raw, ok := params[key]
	if !ok {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return fallback
	}
}

func intParam(params map[string]string, key string, fallback int) int {
	raw, ok := params[key]
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return n
}

type outlineChapter struct {
	Num     int    `json:"num"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type outlineDoc struct {
	Chapters []outlineChapter `json:"chapters"`
}

func parseOutlineJSON(raw string) (*outlineDoc, error) {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("outline json not found")
	}
	var outline outlineDoc
	if err := json.Unmarshal([]byte(raw[start:end+1]), &outline); err != nil {
		return nil, err
	}
	if len(outline.Chapters) == 0 {
		return nil, fmt.Errorf("outline has no chapters")
	}
	return &outline, nil
}

func parseChapterFileNum(name string) (int, bool) {
	base := name
	for _, suf := range []string{".plot.md", ".summary", ".md"} {
		if strings.HasSuffix(base, suf) {
			base = strings.TrimSuffix(base, suf)
			break
		}
	}
	if !strings.HasPrefix(base, "chapter-") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(base, "chapter-"))
	if err != nil {
		return 0, false
	}
	return n, true
}

// BuildIndexFromWorkspaceRoot indexes chapter bodies, plots, summaries and import sources.
func BuildIndexFromWorkspaceRoot(root string) (*Index, error) {
	var chunks []Chunk

	addFile := func(path, source string, chapter int, title string) {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || len(strings.TrimSpace(string(data))) < 20 {
			return
		}
		text := strings.TrimSpace(string(data))
		chunks = append(chunks, Chunk{
			ID:      fmt.Sprintf("%s:%s", source, path),
			Chapter: chapter,
			Title:   title,
			Text:    text,
			Source:  source,
		})
	}

	if raw, err := os.ReadFile(filepath.Join(root, "outline.json")); err == nil {
		if outline, err := parseOutlineJSON(string(raw)); err == nil {
			for _, ch := range outline.Chapters {
				summary := strings.TrimSpace(ch.Summary)
				if summary == "" {
					continue
				}
				chunks = append(chunks, Chunk{
					ID:      fmt.Sprintf("outline:%d", ch.Num),
					Chapter: ch.Num,
					Title:   ch.Title,
					Text:    fmt.Sprintf("第%d章《%s》梗概: %s", ch.Num, ch.Title, summary),
					Source:  "outline",
				})
			}
		}
	}

	chDir := filepath.Join(root, "chapters")
	if entries, err := os.ReadDir(chDir); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if num, ok := parseChapterFileNum(name); ok {
				source := "chapter"
				if strings.HasSuffix(name, ".plot.md") {
					source = "plot"
				} else if strings.HasSuffix(name, ".summary") {
					source = "summary"
				} else if !strings.HasSuffix(name, ".md") {
					continue
				}
				addFile(filepath.Join("chapters", name), source, num, name)
			}
		}
	}

	importDir := filepath.Join(root, "imports", "chapters")
	if entries, err := os.ReadDir(importDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			if num, ok := parseChapterFileNum(e.Name()); ok {
				addFile(filepath.Join("imports", "chapters", e.Name()), "import", num, e.Name())
			}
		}
	}

	if len(chunks) == 0 {
		return &Index{Chunks: []Chunk{}}, nil
	}
	return &Index{Chunks: chunks}, nil
}

// SaveIndexAt writes rag/index.json under workspace root.
func SaveIndexAt(root string, idx *Index) error {
	if idx == nil {
		idx = &Index{Chunks: []Chunk{}}
	}
	raw, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	target := filepath.Join(root, filepath.Clean(IndexPath))
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	return os.WriteFile(target, raw, 0644)
}

// LoadIndexAt reads rag/index.json from workspace root.
func LoadIndexAt(root string) (*Index, error) {
	data, err := os.ReadFile(filepath.Join(root, IndexPath))
	if err != nil {
		return nil, err
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}

// RebuildIndexAt rebuilds and persists the workspace index and optional vectors.
func RebuildIndexAt(root string) (*Index, error) {
	idx, err := BuildIndexFromWorkspaceRoot(root)
	if err != nil {
		return nil, err
	}
	if err := SaveIndexAt(root, idx); err != nil {
		return nil, err
	}
	if err := rebuildVectorsForIndex(root, idx); err != nil {
		return idx, err
	}
	return idx, nil
}

var tokenRE = regexp.MustCompile(`[\p{Han}]|[a-zA-Z0-9]+`)

func tokenize(s string) map[string]int {
	freq := map[string]int{}
	for _, tok := range tokenRE.FindAllString(strings.ToLower(s), -1) {
		if len(tok) < 2 && !unicode.Is(unicode.Han, []rune(tok)[0]) {
			continue
		}
		freq[tok]++
	}
	return freq
}

// SearchAt returns top matching chunks (hybrid when embeddings are configured).
func SearchAt(root, query string, topK int) ([]Chunk, error) {
	mode := "bm25"
	if EmbedderFromEnv().Available() {
		mode = "hybrid"
	}
	return searchAt(root, query, topK, mode)
}