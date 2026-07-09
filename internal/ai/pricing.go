package ai

import (
	"os"
	"strings"
)

// ModelPricing is USD cost per 1M tokens.
type ModelPricing struct {
	PromptPer1M     float64
	CompletionPer1M float64
}

// Default pricing (USD per 1M tokens). Override via TOKEN_COST_MODEL env.
var modelPricingTable = map[string]ModelPricing{
	"deepseek-chat":     {PromptPer1M: 0.27, CompletionPer1M: 1.10},
	"deepseek-reasoner": {PromptPer1M: 0.55, CompletionPer1M: 2.19},
	"grok-4.3":          {PromptPer1M: 3.00, CompletionPer1M: 15.00},
	"grok-3":            {PromptPer1M: 3.00, CompletionPer1M: 15.00},
	"grok-2":            {PromptPer1M: 2.00, CompletionPer1M: 10.00},
	"gpt-4o":            {PromptPer1M: 2.50, CompletionPer1M: 10.00},
	"gpt-4o-mini":       {PromptPer1M: 0.15, CompletionPer1M: 0.60},
}

const defaultCostModel = "deepseek-chat"

// CostModel returns the model used for cost estimation.
func CostModel() string {
	if v := strings.TrimSpace(os.Getenv("TOKEN_COST_MODEL")); v != "" {
		return v
	}
	return defaultCostModel
}

// EstimateCostUSD estimates spend for prompt/completion tokens using the given model.
func EstimateCostUSD(model string, promptTokens, completionTokens int) float64 {
	if promptTokens <= 0 && completionTokens <= 0 {
		return 0
	}
	p, ok := modelPricingTable[normalizeModel(model)]
	if !ok {
		p = modelPricingTable[defaultCostModel]
	}
	return float64(promptTokens)/1_000_000*p.PromptPer1M +
		float64(completionTokens)/1_000_000*p.CompletionPer1M
}

// EstimateCostUSDDefault uses TOKEN_COST_MODEL or deepseek-chat.
func EstimateCostUSDDefault(promptTokens, completionTokens int) float64 {
	return EstimateCostUSD(CostModel(), promptTokens, completionTokens)
}

func normalizeModel(model string) string {
	return strings.ToLower(strings.TrimSpace(model))
}
