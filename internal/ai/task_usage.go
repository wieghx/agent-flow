package ai

import agentflowiov1alpha1 "agent-flow/api/v1alpha1"

// ToTaskTokenUsage converts internal usage to CRD status.
func ToTaskTokenUsage(u TokenUsage) *agentflowiov1alpha1.TokenUsage {
	if u.PromptTokens == 0 && u.CompletionTokens == 0 && u.TotalTokens == 0 {
		return nil
	}
	return &agentflowiov1alpha1.TokenUsage{
		PromptTokens:     int32(u.PromptTokens),
		CompletionTokens: int32(u.CompletionTokens),
		TotalTokens:      int32(u.TotalTokens),
	}
}

// FromTaskTokenUsage converts CRD status to internal usage.
func FromTaskTokenUsage(u *agentflowiov1alpha1.TokenUsage) TokenUsage {
	if u == nil {
		return TokenUsage{}
	}
	return TokenUsage{
		PromptTokens:     int(u.PromptTokens),
		CompletionTokens: int(u.CompletionTokens),
		TotalTokens:      int(u.TotalTokens),
	}
}