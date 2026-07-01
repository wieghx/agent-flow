package workflow

import (
	"fmt"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
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
	eraLine := ""
	if era != "" {
		eraLine = fmt.Sprintf("时代背景锚点: %s\n", era)
	}
	return fmt.Sprintf(`%s

你是历史小说调研编辑。必须使用 MCP 工具联网检索，整理可写入小说的史实与民俗资料。

工作区: %s
输出文件: %s/%s

调研清单（用 historical_research、web_search、wikipedia_search、web_fetch）:
1. 时代与地理：朝代纪年、都城/地域风貌、社会制度
2. 真实人物：身份、关系、可查证的言行典故（标注姓名与称谓）
3. 民俗日常：服饰、饮食、居所、交通、节庆、称谓礼仪
4. 小说边界：哪些可虚构，哪些不得明显违背史料

执行建议:
- 先调用 historical_research（era/location/topics/figures）
- 对关键条目用 web_fetch 补充 1-2 个权威来源
- 用 file_write 将完整 Markdown 写入 %s/%s
- FinalAnswer 输出与文件相同的 Markdown 正文

%s`, wf.Spec.Prompt, ws, ws, ResearchArtifact, ws, ResearchArtifact, eraLine)
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