package workflow

import (
	"fmt"
	"strconv"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/prompts"
)

// ThreeStageEnabled reports whether 梗概→剧情→正文 pipeline is active.
func ThreeStageEnabled(params map[string]string) bool {
	if raw := strings.TrimSpace(params["threeStage"]); raw != "" {
		return BoolParam(params, "threeStage", true)
	}
	return true
}

// PlotsStageActive reports whether the workflow spec includes the plots foreach step.
func PlotsStageActive(wf *agentflowiov1alpha1.Workflow) bool {
	if wf == nil || !ThreeStageEnabled(wf.Spec.Params) {
		return false
	}
	specSteps := wf.Spec.Steps
	if len(specSteps) == 0 && wf.Spec.Template != "" {
		if built, err := BuildSpecFromTemplate(wf.Spec.Template, wf.Spec.Prompt, wf.Spec.Params); err == nil {
			specSteps = built.Steps
		}
	}
	for _, step := range specSteps {
		if step.ID == "plots" {
			return true
		}
	}
	return false
}

// PlotWordsTarget returns target runes for the plot (剧情) layer.
func PlotWordsTarget(params map[string]string) int {
	if v := IntParam(params, "plotWords", 0); v > 0 {
		return v
	}
	return 1000
}

// PlotNumFromStepID parses chapter number from plot-03 style ids.
func PlotNumFromStepID(stepID string) (int, bool) {
	if !strings.HasPrefix(stepID, "plot-") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(stepID, "plot-"))
	if err != nil {
		return 0, false
	}
	return n, true
}

// PlotFileName returns workspace plot script path.
func PlotFileName(num, width int) string {
	if width <= 0 {
		width = 2
	}
	return fmt.Sprintf("chapters/chapter-%0*d.plot.md", width, num)
}

// ReadChapterPlot loads the plot script for a chapter if present.
func ReadChapterPlot(wf *agentflowiov1alpha1.Workflow, num, width int) string {
	raw, err := ReadArtifact(wf, PlotFileName(num, width))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(raw)
}

func plotForeachInstruction(params map[string]string) string {
	target := PlotWordsTarget(params)
	return fmt.Sprintf(`你是小说剧情编剧。根据本章梗概，将大纲扩写为可执行的「剧情脚本」（不是正文）。
目标约 %d 字，须包含：
1. 3-6 个场景节拍（场景地点、参与人物、冲突、转折）
2. 关键对话要点与情绪走向
3. 与上一章的衔接点、本章悬念/钩子
4. 须落实本章梗概，不得跑题或引入未登记主角

只输出剧情脚本文本（可用小标题分场景），不要写成完整散文正文，不要解释写作过程。`, target)
}

// BuildPlotInstruction renders plot-stage worker prompt.
func BuildPlotInstruction(wf *agentflowiov1alpha1.Workflow, base string, outline *NovelOutline, chapter ChapterOutline, context ContextBundle, params map[string]string, bible *StyleBible) string {
	styleBlock := FormatStyleBibleBlock(bible)
	ragBlock := ""
	if wf != nil && outline != nil {
		ragBlock = BuildRAGContextBlock(wf, outline.Title, chapter.Summary)
	}
	return prompts.BuildPlotInstruction(
		base,
		outline.Title,
		outline.Synopsis,
		FormatCharacters(outline),
		chapter.Num,
		chapter.Title,
		chapter.Summary,
		context.RecentSummaries,
		context.PreviousEnding,
		ragBlock,
		styleBlock,
	)
}

func plotForeachDependsOn(outlineDep string) []string {
	return []string{outlineDep}
}

func proseForeachDependsOn(team bool, threeStage bool) []string {
	deps := []string{}
	if threeStage {
		deps = append(deps, "plots")
	} else {
		deps = append(deps, "outline-refine")
	}
	if team {
		deps = append(deps, "style-bible")
	}
	return deps
}
