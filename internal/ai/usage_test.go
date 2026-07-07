package ai

import "testing"

func TestParseUsageFromResponse(t *testing.T) {
	data := map[string]interface{}{
		"usage": map[string]interface{}{
			"prompt_tokens":     float64(120),
			"completion_tokens": float64(450),
			"total_tokens":      float64(570),
		},
	}
	usage := ParseUsageFromResponse(data)
	if usage.PromptTokens != 120 || usage.CompletionTokens != 450 || usage.TotalTokens != 570 {
		t.Fatalf("unexpected usage: %+v", usage)
	}
}

func TestTokenUsageAdd(t *testing.T) {
	var total TokenUsage
	total.Add(TokenUsage{PromptTokens: 100, CompletionTokens: 200, TotalTokens: 300})
	total.Add(TokenUsage{PromptTokens: 50, CompletionTokens: 80, TotalTokens: 130})
	if total.PromptTokens != 150 || total.CompletionTokens != 280 || total.TotalTokens != 430 {
		t.Fatalf("unexpected total: %+v", total)
	}
}