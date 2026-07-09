package workflow

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

const ImportSourcePath = "imports/source.txt"

var importChapterRE = regexp.MustCompile(`(?m)^第\s*([0-9一二三四五六七八九十百千]+)\s*章`)

// ImportedChapter is one parsed chapter from source text.
type ImportedChapter struct {
	Num     int
	Title   string
	Content string
}

// ParseImportedNovel splits raw novel text into chapters.
func ParseImportedNovel(raw string) ([]ImportedChapter, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("import text is empty")
	}
	indices := importChapterRE.FindAllStringIndex(raw, -1)
	if len(indices) == 0 {
		return []ImportedChapter{{
			Num:     1,
			Title:   "第一章",
			Content: raw,
		}}, nil
	}
	var chapters []ImportedChapter
	for i, loc := range indices {
		header := raw[loc[0]:loc[1]]
		start := loc[1]
		end := len(raw)
		if i+1 < len(indices) {
			end = indices[i+1][0]
		}
		body := strings.TrimSpace(raw[start:end])
		num := i + 1
		title := strings.TrimSpace(header)
		chapters = append(chapters, ImportedChapter{Num: num, Title: title, Content: body})
	}
	return chapters, nil
}

// WriteImportedNovel persists source, per-chapter files and stub outline.json.
func WriteImportedNovel(wf *agentflowiov1alpha1.Workflow, raw, bookTitle string) (*NovelOutline, error) {
	if err := WriteArtifact(wf, ImportSourcePath, raw); err != nil {
		return nil, err
	}
	chapters, err := ParseImportedNovel(raw)
	if err != nil {
		return nil, err
	}
	width := ChapterPaddingWidth(len(chapters))
	for _, ch := range chapters {
		path := fmt.Sprintf("imports/chapters/%s", ChapterFileName(ch.Num, width))
		if err := WriteArtifact(wf, path, ch.Content); err != nil {
			return nil, err
		}
	}
	outline := MergeImportedOutline(bookTitle, chapters)
	rawOutline, err := json.MarshalIndent(outline, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := WriteArtifact(wf, "outline.json", string(rawOutline)); err != nil {
		return nil, err
	}
	return outline, nil
}

// MergeImportedOutline builds outline.json from imported chapters (stub summaries).
func MergeImportedOutline(bookTitle string, chapters []ImportedChapter) *NovelOutline {
	if bookTitle == "" {
		bookTitle = "导入小说"
	}
	outline := &NovelOutline{
		Title:    bookTitle,
		Synopsis: "由导入文本拆书生成，待 import-deconstruct 步骤精修人物与梗概。",
	}
	for _, ch := range chapters {
		summary := SummarizeChapter(ch.Content, 200)
		outline.Chapters = append(outline.Chapters, ChapterOutline{
			Num:     ch.Num,
			Title:   ch.Title,
			Summary: summary,
		})
	}
	return outline
}

// ImportDeconstructInstruction is the AI prompt for 拆书.
func ImportDeconstructInstruction(bookTitle string, chapterCount int) string {
	return fmt.Sprintf(`书名: %s

读取工作区 imports/source.txt 与 outline.json（各章为导入拆分的初稿梗概）。
请完成「拆书」并输出严格 JSON（不要 markdown 代码块）：
{
  "title": "书名",
  "synopsis": "全书简介（200字内）",
  "characters": [{"name":"角色名","role":"定位","trait":"特征"}],
  "chapters": [{"num":1,"title":"章节标题","summary":"精炼梗概（冲突+转折+衔接）"}]
}
要求：
1. chapters 必须恰好 %d 章，num 连续
2. 从正文提炼真实人物关系，不得臆造主角
3. 每章 summary 比初稿更精炼，保留关键冲突`, bookTitle, chapterCount)
}
