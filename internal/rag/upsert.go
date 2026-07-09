package rag

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func chunkFromRelativePath(root, relPath string) (*Chunk, error) {
	relPath = filepath.Clean(relPath)
	full := filepath.Join(root, relPath)
	data, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	text := strings.TrimSpace(string(data))
	if len(text) < 20 {
		return nil, fmt.Errorf("artifact too short: %s", relPath)
	}

	source := "artifact"
	chapter := 0
	title := filepath.Base(relPath)
	if strings.HasPrefix(relPath, "chapters/") {
		name := filepath.Base(relPath)
		if num, ok := parseChapterFileNum(name); ok {
			chapter = num
			switch {
			case strings.HasSuffix(name, ".plot.md"):
				source = "plot"
			case strings.HasSuffix(name, ".summary"):
				source = "summary"
			default:
				source = "chapter"
			}
		}
	} else if strings.HasPrefix(relPath, "imports/chapters/") {
		source = "import"
		if num, ok := parseChapterFileNum(filepath.Base(relPath)); ok {
			chapter = num
		}
	}

	return &Chunk{
		ID:      fmt.Sprintf("%s:%s", source, relPath),
		Chapter: chapter,
		Title:   title,
		Text:    text,
		Source:  source,
	}, nil
}

func outlineChunksFromRoot(root string) ([]Chunk, error) {
	raw, err := os.ReadFile(filepath.Join(root, "outline.json"))
	if err != nil {
		return nil, err
	}
	outline, err := parseOutlineJSON(string(raw))
	if err != nil {
		return nil, err
	}
	var chunks []Chunk
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
	return chunks, nil
}

func removeChunksByFilter(chunks []Chunk, keep func(Chunk) bool) []Chunk {
	var out []Chunk
	for _, ch := range chunks {
		if keep(ch) {
			out = append(out, ch)
		}
	}
	return out
}

func upsertChunks(idx *Index, incoming []Chunk) {
	if idx == nil {
		return
	}
	if len(incoming) == 0 {
		return
	}
	removeIDs := make(map[string]bool, len(incoming))
	for _, ch := range incoming {
		removeIDs[ch.ID] = true
	}
	filtered := removeChunksByFilter(idx.Chunks, func(ch Chunk) bool {
		return !removeIDs[ch.ID]
	})
	idx.Chunks = append(filtered, incoming...)
}

func loadOrEmptyIndex(root string) (*Index, error) {
	idx, err := LoadIndexAt(root)
	if err != nil {
		if os.IsNotExist(err) {
			return &Index{Chunks: []Chunk{}}, nil
		}
		return nil, err
	}
	return idx, nil
}

func loadOrEmptyVectors(root string) (*VectorStore, error) {
	store, err := LoadVectorsAt(root)
	if err != nil {
		if os.IsNotExist(err) {
			return &VectorStore{Vectors: map[string][]float32{}}, nil
		}
		return nil, err
	}
	return store, nil
}

func upsertVectors(root string, chunks []Chunk) error {
	emb := EmbedderFromEnv()
	if !emb.Available() || len(chunks) == 0 {
		return nil
	}
	vecs, err := embedChunks(context.Background(), emb, chunks)
	if err != nil {
		return err
	}
	store, err := loadOrEmptyVectors(root)
	if err != nil {
		return err
	}
	store.Model = emb.Model()
	for id, vec := range vecs {
		store.Vectors[id] = vec
	}
	return SaveVectorsAt(root, store)
}

// UpsertArtifactAt incrementally updates one workspace artifact in the RAG index.
func UpsertArtifactAt(root, relPath string) error {
	chunk, err := chunkFromRelativePath(root, relPath)
	if err != nil {
		return err
	}
	idx, err := loadOrEmptyIndex(root)
	if err != nil {
		return err
	}
	upsertChunks(idx, []Chunk{*chunk})
	if err := SaveIndexAt(root, idx); err != nil {
		return err
	}
	return upsertVectors(root, []Chunk{*chunk})
}

// RefreshOutlineAt replaces outline-derived chunks after outline.json changes.
func RefreshOutlineAt(root string) error {
	outlineChunks, err := outlineChunksFromRoot(root)
	if err != nil {
		return err
	}
	idx, err := loadOrEmptyIndex(root)
	if err != nil {
		return err
	}
	idx.Chunks = removeChunksByFilter(idx.Chunks, func(ch Chunk) bool {
		return ch.Source != "outline"
	})
	idx.Chunks = append(idx.Chunks, outlineChunks...)
	if err := SaveIndexAt(root, idx); err != nil {
		return err
	}
	return upsertVectors(root, outlineChunks)
}
