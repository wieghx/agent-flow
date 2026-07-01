package rag

import (
	"context"
	"sort"
	"strings"
)

// SearchMode resolves retrieval strategy from workflow params.
func SearchMode(params map[string]string) string {
	mode := strings.ToLower(strings.TrimSpace(params["ragSearchMode"]))
	switch mode {
	case "bm25", "vector", "hybrid":
		return mode
	}
	if EmbedderFromEnv().Available() {
		return "hybrid"
	}
	return "bm25"
}

// SearchAtForParams runs retrieval with workflow param overrides.
func SearchAtForParams(root, query string, topK int, params map[string]string) ([]Chunk, error) {
	return searchAt(root, query, topK, SearchMode(params))
}

func searchAt(root, query string, topK int, mode string) ([]Chunk, error) {
	if topK <= 0 {
		topK = 5
	}
	idx, err := LoadIndexAt(root)
	if err != nil {
		if rebuilt, rbErr := RebuildIndexAt(root); rbErr == nil {
			idx = rebuilt
		} else {
			return nil, err
		}
	}
	if len(idx.Chunks) == 0 {
		return nil, nil
	}

	bm25 := bm25Scores(idx.Chunks, query)
	emb := EmbedderFromEnv()
	useVector := emb.Available() && (mode == "vector" || mode == "hybrid")
	var vecScores map[string]float64
	if useVector {
		vecScores = vectorScores(root, idx.Chunks, query, emb)
	}

	type ranked struct {
		chunk Chunk
		score float64
	}
	var rankedList []ranked
	for _, ch := range idx.Chunks {
		var score float64
		switch mode {
		case "vector":
			score = vecScores[ch.ID]
		case "hybrid":
			score = 0.45*bm25[ch.ID] + 0.55*vecScores[ch.ID]
		default:
			score = bm25[ch.ID]
		}
		if score > 0 {
			rankedList = append(rankedList, ranked{chunk: ch, score: score})
		}
	}
	sort.Slice(rankedList, func(i, j int) bool { return rankedList[i].score > rankedList[j].score })
	if len(rankedList) > topK {
		rankedList = rankedList[:topK]
	}
	out := make([]Chunk, len(rankedList))
	for i, r := range rankedList {
		out[i] = r.chunk
	}
	return out, nil
}

func bm25Scores(chunks []Chunk, query string) map[string]float64 {
	qFreq := tokenize(query)
	if len(qFreq) == 0 {
		return map[string]float64{}
	}
	scores := make(map[string]float64, len(chunks))
	var maxScore float64
	for _, ch := range chunks {
		cFreq := tokenize(ch.Text)
		var score float64
		for tok, qv := range qFreq {
			if cv, ok := cFreq[tok]; ok {
				score += float64(qv*cv) / float64(len(cFreq)+1)
			}
		}
		if score > maxScore {
			maxScore = score
		}
		scores[ch.ID] = score
	}
	if maxScore > 0 {
		for id, s := range scores {
			scores[id] = s / maxScore
		}
	}
	return scores
}

func vectorScores(root string, chunks []Chunk, query string, emb Embedder) map[string]float64 {
	scores := make(map[string]float64, len(chunks))
	store, err := LoadVectorsAt(root)
	if err != nil || len(store.Vectors) == 0 {
		return scores
	}
	qVec, err := emb.Embed(context.Background(), []string{query})
	if err != nil || len(qVec) == 0 {
		return scores
	}
	var maxScore float64
	for _, ch := range chunks {
		vec, ok := store.Vectors[ch.ID]
		if !ok {
			continue
		}
		s := cosineSimilarity(qVec[0], vec)
		if s > maxScore {
			maxScore = s
		}
		scores[ch.ID] = s
	}
	if maxScore > 0 {
		for id, s := range scores {
			if s > 0 {
				scores[id] = s / maxScore
			}
		}
	}
	return scores
}