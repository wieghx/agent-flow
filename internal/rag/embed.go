package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const VectorsPath = "rag/vectors.json"

// VectorStore persists chunk embeddings on disk.
type VectorStore struct {
	Model   string               `json:"model,omitempty"`
	Vectors map[string][]float32 `json:"vectors"`
}

// Embedder produces text embeddings.
type Embedder interface {
	Available() bool
	Model() string
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type openAICompatEmbedder struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

type noopEmbedder struct{}

func (noopEmbedder) Available() bool                                      { return false }
func (noopEmbedder) Model() string                                        { return "" }
func (noopEmbedder) Embed(context.Context, []string) ([][]float32, error) { return nil, nil }

var (
	embedderOnce sync.Once
	embedderInst Embedder
)

// EmbedderFromEnv returns a cached OpenAI-compatible embedder or a noop instance.
func EmbedderFromEnv() Embedder {
	embedderOnce.Do(func() {
		base := strings.TrimSpace(os.Getenv("RAG_EMBEDDING_BASE_URL"))
		if base == "" {
			base = strings.TrimSpace(os.Getenv("AI_BASE_URL"))
		}
		key := strings.TrimSpace(os.Getenv("RAG_EMBEDDING_API_KEY"))
		if key == "" {
			key = strings.TrimSpace(os.Getenv("AI_API_KEY"))
		}
		model := strings.TrimSpace(os.Getenv("RAG_EMBEDDING_MODEL"))
		if model == "" {
			model = "text-embedding-3-small"
		}
		if base == "" || key == "" {
			embedderInst = noopEmbedder{}
			return
		}
		base = strings.TrimRight(base, "/")
		embedderInst = &openAICompatEmbedder{
			baseURL: base,
			apiKey:  key,
			model:   model,
			client:  &http.Client{Timeout: 60 * time.Second},
		}
	})
	return embedderInst
}

func (e *openAICompatEmbedder) Available() bool { return e != nil && e.baseURL != "" && e.apiKey != "" }
func (e *openAICompatEmbedder) Model() string   { return e.model }

func (e *openAICompatEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(map[string]interface{}{
		"model": e.model,
		"input": texts,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/v1/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.apiKey)

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings API %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([][]float32, len(texts))
	for _, item := range parsed.Data {
		if item.Index < 0 || item.Index >= len(texts) {
			continue
		}
		vec := make([]float32, len(item.Embedding))
		for i, v := range item.Embedding {
			vec[i] = float32(v)
		}
		out[item.Index] = vec
	}
	for i, vec := range out {
		if vec == nil {
			return nil, fmt.Errorf("missing embedding for input %d", i)
		}
	}
	return out, nil
}

// LoadVectorsAt reads rag/vectors.json.
func LoadVectorsAt(root string) (*VectorStore, error) {
	data, err := os.ReadFile(vectorFilePath(root))
	if err != nil {
		return nil, err
	}
	var store VectorStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	if store.Vectors == nil {
		store.Vectors = map[string][]float32{}
	}
	return &store, nil
}

// SaveVectorsAt writes rag/vectors.json.
func SaveVectorsAt(root string, store *VectorStore) error {
	if store == nil {
		store = &VectorStore{Vectors: map[string][]float32{}}
	}
	if store.Vectors == nil {
		store.Vectors = map[string][]float32{}
	}
	raw, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	target := vectorFilePath(root)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	return os.WriteFile(target, raw, 0644)
}

func vectorFilePath(root string) string {
	return strings.TrimRight(root, "/") + "/" + VectorsPath
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		af, bf := float64(a[i]), float64(b[i])
		dot += af * bf
		na += af * af
		nb += bf * bf
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

func embedChunks(ctx context.Context, emb Embedder, chunks []Chunk) (map[string][]float32, error) {
	if emb == nil || !emb.Available() || len(chunks) == 0 {
		return nil, nil
	}
	texts := make([]string, len(chunks))
	for i, ch := range chunks {
		texts[i] = ch.Text
	}
	vecs, err := emb.Embed(ctx, texts)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]float32, len(chunks))
	for i, ch := range chunks {
		out[ch.ID] = vecs[i]
	}
	return out, nil
}

func rebuildVectorsForIndex(root string, idx *Index) error {
	emb := EmbedderFromEnv()
	if !emb.Available() || idx == nil {
		return nil
	}
	vecs, err := embedChunks(context.Background(), emb, idx.Chunks)
	if err != nil {
		return err
	}
	store := &VectorStore{Model: emb.Model(), Vectors: vecs}
	return SaveVectorsAt(root, store)
}
