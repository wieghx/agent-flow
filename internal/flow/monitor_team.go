package flow

import (
	"context"
	"fmt"
	"strings"

	"agent-flow/internal/ai"
	wfengine "agent-flow/internal/workflow"
)

func evaluateNovelStyleBibleRules(output string, issues []string, score int) (int, []string) {
	if _, err := wfengine.ParseStyleBibleJSON(output); err != nil {
		issues = append(issues, "style_bible_invalid_json")
		score -= 40
	}
	return score, issues
}

func evaluateTeamChapterRules(instruction, output string, issues []string, score int) (int, []string) {
	prev := extractLabeledSection(instruction, "上一章结尾")
	if prev != "" {
		opening := wfengine.ChapterOpening(output, 320)
		if wfengine.ContinuityWeak(prev, opening) {
			issues = append(issues, "continuity_break")
			score -= 25
		}
	}
	if names := wfengine.ProtagonistNamesFromInstruction(instruction); len(names) > 0 {
		mentioned := 0
		for _, name := range names {
			if strings.Contains(output, name) {
				mentioned++
			}
		}
		if mentioned == 0 {
			issues = append(issues, "bible_protagonist_missing")
			score -= 30
		}
	}
	return score, issues
}

func evaluateCanonGate(ctx context.Context, aiSvc *ai.Service, instruction, output string, threshold, attempt int) (*EvalResult, error) {
	systemPrompt := fmt.Sprintf(`你是设定监工（Canon Gate），只做设定与衔接快检。

检查：主角姓名是否与设定圣经/大纲一致？剧情是否符合本章梗概？与上一章是否衔接？
通过阈值: %d 分

仅返回 JSON：
{"score": 分数, "passed": true/false, "feedback": "一句中文评价", "issues": ["character_rename","plot_jump","continuity_break"]}`, threshold)
	return EvaluateWithAI(ctx, aiSvc, systemPrompt, instruction, output, nil, threshold, attempt)
}

func extractLabeledSection(instruction, label string) string {
	markers := []string{label + "（", label + "(", label + ":", label + "："}
	for _, line := range strings.Split(instruction, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, label) {
			continue
		}
		for _, m := range markers {
			if strings.HasPrefix(trimmed, m) || trimmed == label {
				if idx := strings.Index(trimmed, "："); idx >= 0 {
					return strings.TrimSpace(trimmed[idx+len("："):])
				}
				if idx := strings.Index(trimmed, ":"); idx >= 0 {
					return strings.TrimSpace(trimmed[idx+1:])
				}
			}
		}
		return strings.TrimSpace(strings.TrimLeft(strings.TrimPrefix(trimmed, label), "：（: "))
	}
	return ""
}
