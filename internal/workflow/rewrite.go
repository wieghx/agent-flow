package workflow

import (
	"fmt"
	"strconv"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/prompts"
)

const (
	RewriteLayerPlot     = "plot"
	RewriteLayerChapter  = "chapter"
	ParamSharedWorkspace = "sharedWorkspace"
	ParamRewriteChapter  = "rewriteChapterNum"
	ParamRewriteLayer    = "rewriteLayer"
	ParamRewriteNote     = "rewriteInstruction"
	ParamParentWorkflow  = "parentWorkflow"
)

// RewriteChapterNum reads target chapter number from rewrite workflow params.
func RewriteChapterNum(params map[string]string) int {
	return IntParam(params, ParamRewriteChapter, 0)
}

// RewriteLayer returns plot or chapter for a rewrite workflow.
func RewriteLayer(params map[string]string) string {
	layer := strings.ToLower(strings.TrimSpace(params[ParamRewriteLayer]))
	if layer == RewriteLayerPlot {
		return RewriteLayerPlot
	}
	return RewriteLayerChapter
}

// RewriteInstruction returns the user's modification request.
func RewriteInstruction(params map[string]string) string {
	return strings.TrimSpace(params[ParamRewriteNote])
}

// RewriteOutputPath returns workspace path for the rewrite artifact.
func RewriteOutputPath(params map[string]string) string {
	num := RewriteChapterNum(params)
	if num <= 0 {
		return ""
	}
	width := 2
	if raw := strings.TrimSpace(params["chapterCount"]); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			width = ChapterPaddingWidth(n)
		}
	}
	if RewriteLayer(params) == RewriteLayerPlot {
		return PlotFileName(num, width)
	}
	return fmt.Sprintf("chapters/%s", ChapterFileName(num, width))
}

// BuildRewriteInstruction renders the worker prompt for chapter/plot rewrite.
func BuildRewriteInstruction(wf *agentflowiov1alpha1.Workflow) (string, error) {
	if wf == nil {
		return "", fmt.Errorf("workflow is nil")
	}
	params := wf.Spec.Params
	num := RewriteChapterNum(params)
	if num <= 0 {
		return "", fmt.Errorf("rewriteChapterNum is required")
	}
	layer := RewriteLayer(params)
	note := RewriteInstruction(params)
	if note == "" {
		return "", fmt.Errorf("rewriteInstruction is required")
	}

	outline, err := LoadOutline(wf)
	if err != nil {
		return "", err
	}
	width := ChapterPaddingWidth(len(outline.Chapters))
	var chapter ChapterOutline
	for _, ch := range outline.Chapters {
		if ch.Num == num {
			chapter = ch
			break
		}
	}
	if chapter.Num == 0 {
		return "", fmt.Errorf("chapter %d not found in outline", num)
	}

	base, err := prompts.BuildRewriteInstruction(wf.Spec.Prompt, num, layer, note)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString(base)
	if bible, _ := LoadStyleBible(wf); bible != nil {
		if block := FormatStyleBibleBlock(bible); block != "" {
			b.WriteString("\n\n")
			b.WriteString(block)
		}
	}
	fmt.Fprintf(&b, "\n当前章节: 第%d章《%s》\n", chapter.Num, chapter.Title)
	fmt.Fprintf(&b, "本章梗概: %s\n", chapter.Summary)

	if layer == RewriteLayerPlot {
		if existing := ReadChapterPlot(wf, num, width); existing != "" {
			b.WriteString("\n【当前剧情脚本】\n")
			b.WriteString(existing)
			b.WriteString("\n")
		}
	} else {
		path := fmt.Sprintf("chapters/%s", ChapterFileName(num, width))
		if raw, err := ReadArtifact(wf, path); err == nil && strings.TrimSpace(raw) != "" {
			b.WriteString("\n【当前正文】\n")
			b.WriteString(strings.TrimSpace(raw))
			b.WriteString("\n")
		}
		if plot := ReadChapterPlot(wf, num, width); plot != "" {
			b.WriteString("\n【本章剧情脚本（须保持一致）】\n")
			b.WriteString(plot)
			b.WriteString("\n")
		}
	}

	if block := BuildRAGContextBlock(wf, outline.Title, chapter.Summary+" "+note); block != "" {
		b.WriteString("\n")
		b.WriteString(block)
		b.WriteString("\n")
	}

	b.WriteString("\n【作者修改意见】\n")
	b.WriteString(note)
	b.WriteString("\n\n要求：\n")
	b.WriteString("1. 保留未提及的人物、时间线与既有设定\n")
	b.WriteString("2. 只改作者要求的部分，其余保持连贯\n")
	b.WriteString("3. 不得引入未登记主角或元评论\n")
	if layer == RewriteLayerPlot {
		fmt.Fprintf(&b, "4. 输出约 %d 字的剧情脚本，不要写成完整散文正文\n", PlotWordsTarget(params))
	} else {
		words := IntParam(params, "wordsPerChapter", 2500)
		fmt.Fprintf(&b, "4. 输出约 %d 字章节正文，只输出正文（可含章节标题）\n", words)
	}
	return b.String(), nil
}

// BuildOutlineSyncInstruction prompts updating outline summary after rewrite.
func BuildOutlineSyncInstruction(chapterNum int) string {
	return fmt.Sprintf(`读取工作区 outline.json 以及第 %d 章最新剧情/正文文件。
根据改写后的内容，更新该章 summary（冲突+转折+衔接），保持其余章节与字段不变。
严格输出完整 JSON（不要 markdown 代码块），格式与原 outline 一致：
{"title":"","synopsis":"","characters":[],"chapters":[{"num":1,"title":"","summary":""}]}`, chapterNum)
}

// RewriteMonitorType picks QC rubric for rewrite layer.
func RewriteMonitorType(params map[string]string) string {
	if RewriteLayer(params) == RewriteLayerPlot {
		return "novel-plot"
	}
	if TeamModeEnabled(params) {
		return "novel-chapter-team"
	}
	return "novel-chapter"
}
