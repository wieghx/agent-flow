package workflow

import (
	"fmt"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/prompts"
)

const ResearchArtifact = "research_notes.md"

// HistoricalResearchEnabled reports whether to run MCP web research before outline.
func HistoricalResearchEnabled(params map[string]string, prompt string) bool {
	if params != nil {
		if BoolParam(params, "historicalResearch", false) {
			return true
		}
		if era := strings.TrimSpace(params["historicalEra"]); era != "" {
			return true
		}
	}
	return LooksLikeHistoricalIntent(prompt)
}

// LooksLikeHistoricalIntent detects historical fiction requests from user text.
func LooksLikeHistoricalIntent(text string) bool {
	lower := strings.ToLower(text)
	keywords := []string{
		"历史小说", "历史文", "古装", "朝代", "宫廷", "武侠历史", "三国", "唐朝", "宋代", "明朝", "清朝",
		"秦汉", "魏晋", "南北朝", "隋唐", "宋元", "明清", "民国", "架空历史", "考据", "史实",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// HistoricalEraFromParams extracts era hint for research queries.
func HistoricalEraFromParams(params map[string]string, prompt string) string {
	if params != nil {
		if era := strings.TrimSpace(params["historicalEra"]); era != "" {
			return era
		}
	}
	return extractEraHint(prompt)
}

func extractEraHint(prompt string) string {
	markers := []string{"唐朝", "宋代", "明朝", "清朝", "秦汉", "三国", "民国", "开元", "贞观", "康熙", "乾隆"}
	for _, m := range markers {
		if strings.Contains(prompt, m) {
			return m
		}
	}
	return ""
}

// BuildHistoricalResearchInstruction builds the MCP ReAct prompt for background research.
func BuildHistoricalResearchInstruction(wf *agentflowiov1alpha1.Workflow) string {
	ws := WorkspacePath(wf)
	era := HistoricalEraFromParams(wf.Spec.Params, wf.Spec.Prompt)
	return prompts.BuildHistoricalResearchInstruction(wf.Spec.Prompt, era, ws, ws)
}

// ResearchContextBlock returns instruction appendix when research_notes.md exists.
func ResearchContextBlock(wf *agentflowiov1alpha1.Workflow) string {
	raw, err := ReadArtifact(wf, ResearchArtifact)
	if err != nil || strings.TrimSpace(raw) == "" {
		return ""
	}
	summary := SummarizeArc(strings.Split(raw, "\n"), 2000)
	return fmt.Sprintf(`
【历史调研笔记 — 写作须参考】
%s

要求: 人物称谓、时代器物、社会制度须与调研笔记一致；可艺术加工细节，但不得与史料常识明显冲突。`, summary)
}
