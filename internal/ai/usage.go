package ai

// TokenUsage holds LLM token consumption from a single API call.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatResult is the content plus token usage from one Chat call.
type ChatResult struct {
	Content string
	Usage   TokenUsage
}

// Add accumulates another usage into this one.
func (u *TokenUsage) Add(other TokenUsage) {
	u.PromptTokens += other.PromptTokens
	u.CompletionTokens += other.CompletionTokens
	u.TotalTokens += other.TotalTokens
}

// ParseUsageFromResponse extracts usage from OpenAI-compatible JSON responses.
func ParseUsageFromResponse(data map[string]interface{}) TokenUsage {
	raw, ok := data["usage"].(map[string]interface{})
	if !ok {
		return TokenUsage{}
	}
	usage := TokenUsage{
		PromptTokens:     intFromAny(raw["prompt_tokens"]),
		CompletionTokens: intFromAny(raw["completion_tokens"]),
		TotalTokens:      intFromAny(raw["total_tokens"]),
	}
	if usage.TotalTokens == 0 && (usage.PromptTokens > 0 || usage.CompletionTokens > 0) {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage
}

func intFromAny(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}
