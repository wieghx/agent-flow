package workflow

import (
	"fmt"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/prompts"
	"agent-flow/internal/rag"
)

// BuildRAGContextBlock retrieves relevant snippets for prompt injection.
func BuildRAGContextBlock(wf *agentflowiov1alpha1.Workflow, title, query string) string {
	if wf == nil || !RAGEnabled(wf.Spec.Params) {
		return ""
	}
	chunks, err := rag.SearchAtForParams(
		WorkspacePath(wf),
		strings.TrimSpace(title+" "+query),
		rag.TopK(wf.Spec.Params),
		wf.Spec.Params,
	)
	if err != nil || len(chunks) == 0 {
		return ""
	}
	var texts []string
	for _, ch := range chunks {
		text := ch.Text
		if len([]rune(text)) > 600 {
			text = string([]rune(text)[:600]) + "…"
		}
		texts = append(texts, fmt.Sprintf("[%s] %s", ch.Source, text))
	}
	return prompts.BuildRAGContextBlock(title, strings.TrimSpace(title+" "+query), texts)
}

// RAGEnabled is a thin wrapper for workflow params.
func RAGEnabled(params map[string]string) bool {
	return rag.RAGEnabled(params)
}
