package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

// ChapterOutline is one chapter entry in outline.json.
type ChapterOutline struct {
	Num     int    `json:"num"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

// NovelOutline is the parsed outline artifact.
type NovelOutline struct {
	Title      string           `json:"title"`
	Synopsis   string           `json:"synopsis"`
	Characters []map[string]any `json:"characters"`
	Chapters   []ChapterOutline `json:"chapters"`
}

// ParseOutlineJSON extracts novel outline from model output.
func ParseOutlineJSON(raw string) (*NovelOutline, error) {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("outline json not found")
	}
	var outline NovelOutline
	if err := json.Unmarshal([]byte(raw[start:end+1]), &outline); err != nil {
		return nil, err
	}
	if len(outline.Chapters) == 0 {
		return nil, fmt.Errorf("outline has no chapters")
	}
	sort.Slice(outline.Chapters, func(i, j int) bool {
		return outline.Chapters[i].Num < outline.Chapters[j].Num
	})
	return &outline, nil
}

// MarshalOutlineJSON pretty-prints outline for workspace storage.
func MarshalOutlineJSON(outline *NovelOutline) (string, error) {
	if outline == nil {
		return "", fmt.Errorf("outline is nil")
	}
	data, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data) + "\n", nil
}

// ValidateOutlineChapterCount checks outline size against expected count.
func ValidateOutlineChapterCount(outline *NovelOutline, expected int) error {
	if expected <= 0 || outline == nil {
		return nil
	}
	if len(outline.Chapters) < expected {
		return fmt.Errorf("outline has %d chapters, expected %d", len(outline.Chapters), expected)
	}
	return nil
}

// FormatCharacters renders character bible for prompt context.
func FormatCharacters(outline *NovelOutline) string {
	if outline == nil || len(outline.Characters) == 0 {
		return ""
	}
	var lines []string
	for _, ch := range outline.Characters {
		name, _ := ch["name"].(string)
		role, _ := ch["role"].(string)
		trait, _ := ch["trait"].(string)
		if name == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s（%s）: %s", name, role, trait))
	}
	return strings.Join(lines, "\n")
}

// ChapterEnding returns the tail of previous chapter for continuity.
// ChapterOpening returns the first maxLen runes of chapter prose for style anchoring.
func ChapterOpening(content string, maxLen int) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[:maxLen])
}

func ChapterEnding(content string, maxLen int) string {
	content = strings.TrimSpace(content)
	if content == "" || maxLen <= 0 {
		return ""
	}
	runes := []rune(content)
	if len(runes) <= maxLen {
		return content
	}
	return string(runes[len(runes)-maxLen:])
}

// BuildChapterInstruction renders a foreach chapter prompt.
func BuildChapterInstruction(wf *agentflowiov1alpha1.Workflow, base string, outline *NovelOutline, chapter ChapterOutline, context ContextBundle, wordsPerChapter int, bible *StyleBible, width int) string {
	var b strings.Builder
	if block := FormatStyleBibleBlock(bible); block != "" {
		b.WriteString(block)
		b.WriteString("\n\n")
	}
	b.WriteString(base)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "书名: %s\n", outline.Title)
	fmt.Fprintf(&b, "全书简介: %s\n", outline.Synopsis)
	if chars := FormatCharacters(outline); chars != "" {
		b.WriteString("\n主要人物（必须保持一致）:\n")
		b.WriteString(chars)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n当前章节: 第%d章《%s》\n", chapter.Num, chapter.Title)
	fmt.Fprintf(&b, "本章梗概: %s\n", chapter.Summary)
	if wf != nil && ThreeStageEnabled(wf.Spec.Params) {
		if plot := ReadChapterPlot(wf, chapter.Num, width); plot != "" {
			b.WriteString("\n本章剧情脚本（据此写正文，不得偏离）:\n")
			b.WriteString(plot)
			b.WriteString("\n")
		}
	}
	if block := BuildRAGContextBlock(wf, outline.Title, chapter.Summary); block != "" {
		b.WriteString("\n")
		b.WriteString(block)
		b.WriteString("\n")
	}
	if context.ArcSummaries != "" {
		b.WriteString("\n已完成故事弧摘要:\n")
		b.WriteString(context.ArcSummaries)
		b.WriteString("\n")
	}
	if context.StorySoFar != "" {
		b.WriteString("\n更早剧情概要:\n")
		b.WriteString(context.StorySoFar)
		b.WriteString("\n")
	}
	if context.RecentSummaries != "" {
		b.WriteString("\n近几章摘要:\n")
		b.WriteString(context.RecentSummaries)
		b.WriteString("\n")
	}
	if context.PreviousEnding != "" {
		b.WriteString("\n上一章结尾（必须自然衔接）:\n")
		b.WriteString(context.PreviousEnding)
		b.WriteString("\n")
	}
	if wordsPerChapter > 0 {
		fmt.Fprintf(&b, "\n目标字数约: %d 字（正文不少于 %d 字，须完整收束）\n", wordsPerChapter, MinChapterLength(wordsPerChapter))
	}
	b.WriteString("\n要求: 人物性格、时间线、伏笔与上文一致；本章需推进剧情并留下合理悬念；写足篇幅，不要草草收尾。")
	return b.String()
}

// ContextBundle carries rolling context for long novels.
type ContextBundle struct {
	ArcSummaries    string
	StorySoFar      string
	RecentSummaries string
	PreviousEnding  string
}

// SummarizeChapter produces a short summary for context chaining.
func SummarizeChapter(content string, maxLen int) string {
	content = strings.TrimSpace(content)
	content = regexp.MustCompile(`\s+`).ReplaceAllString(content, " ")
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// ChapterNumFromStepID parses chapter number from step id like chapter-03.
func ChapterNumFromStepID(stepID string) (int, bool) {
	if !strings.HasPrefix(stepID, "chapter-") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(stepID, "chapter-"))
	if err != nil {
		return 0, false
	}
	return n, true
}
