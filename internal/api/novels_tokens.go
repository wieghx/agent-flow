package api

import (
	"net/http"

	"agent-flow/internal/ai"
	"agent-flow/internal/store"
)

func (a *API) handleTokenReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	report, err := store.BuildTokenReportFromStore(r.Context(), a.novelStore)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if report == nil {
		report = &store.TokenReport{}
	}
	enrichTokenReportCost(report)
	writeJSON(w, Response{Success: true, Data: report})
}

func enrichTokenReportCost(report *store.TokenReport) {
	if report == nil {
		return
	}
	model := ai.CostModel()
	report.CostModel = model
	report.EstimatedCostUSD = ai.EstimateCostUSD(model, report.PromptTokens, report.CompletionTokens)
	for i := range report.Novels {
		n := &report.Novels[i]
		n.EstimatedCostUSD = ai.EstimateCostUSD(model, n.PromptTokens, n.CompletionTokens)
	}
}

// ChapterSummary is one chapter with token stats for the library API.
type ChapterSummary struct {
	Num              int    `json:"num"`
	Title            string `json:"title"`
	Summary          string `json:"summary,omitempty"`
	Status           string `json:"status"`
	WordCount        int    `json:"word_count"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
}

func (a *API) handleNovelChapters(w http.ResponseWriter, r *http.Request, namespace, name string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	chapters, err := store.ListChaptersFromStore(r.Context(), a.novelStore, namespace, name)
	if err != nil {
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	out := make([]ChapterSummary, 0, len(chapters))
	for _, ch := range chapters {
		out = append(out, ChapterSummary{
			Num:              ch.Num,
			Title:            ch.Title,
			Summary:          ch.Summary,
			Status:           ch.Status,
			WordCount:        ch.WordCount,
			PromptTokens:     ch.PromptTokens,
			CompletionTokens: ch.CompletionTokens,
			TotalTokens:      ch.TotalTokens,
		})
	}
	writeJSON(w, Response{
		Success: true,
		Data: map[string]interface{}{
			"count":    len(out),
			"chapters": out,
		},
	})
}