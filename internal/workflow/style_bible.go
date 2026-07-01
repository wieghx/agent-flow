package workflow

import (
	"encoding/json"
	"fmt"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

const StyleBibleArtifact = "style_bible.json"

// StyleBible is the canonical writing contract for a novel workflow.
type StyleBible struct {
	Title            string   `json:"title"`
	POV              string   `json:"pov"`
	Tone             string   `json:"tone"`
	ProseStyle       string   `json:"prose_style"`
	Protagonists     []string `json:"protagonists"`
	SupportingCast   []string `json:"supporting_cast"`
	Forbidden        []string `json:"forbidden"`
	TimelineAnchor   string   `json:"timeline_anchor"`
	OpeningHookStyle string   `json:"opening_hook_style"`
	ChapterRhythm    string   `json:"chapter_rhythm"`
}

// BuildStyleBibleInstruction renders the设定官 worker prompt from outline.
func BuildStyleBibleInstruction(prompt string, outline *NovelOutline) string {
	var b strings.Builder
	b.WriteString(prompt)
	b.WriteString(`

你是小说设定官（Canon Keeper）。请根据下方大纲生成「设定圣经」JSON，作为全书写作的唯一约束（不要 markdown 代码块）：
{
  "title": "书名",
  "pov": "叙事人称（如：第三人称限知，紧跟主角）",
  "tone": "整体基调（如：冷峻写实、悬疑压迫）",
  "prose_style": "文笔规范（句式、修辞、禁用风格）",
  "protagonists": ["主角姓名，必须与大纲一致"],
  "supporting_cast": ["重要配角姓名"],
  "forbidden": ["严禁事项：换主角名、元评论、网络梗、提纲句等"],
  "timeline_anchor": "时代/地点/时间线锚点",
  "opening_hook_style": "章节开篇惯例",
  "chapter_rhythm": "章节节奏（场景-对话-心理比例）"
}
要求：
1. protagonists 必须来自大纲 characters，不得自创新主角
2. forbidden 须明确「不得更换主角姓名」「不得引入未登记主要角色」
3. 直接以 { 开头、以 } 结尾`)
	if outline != nil {
		b.WriteString("\n\n【已生成大纲】\n")
		fmt.Fprintf(&b, "书名: %s\n", outline.Title)
		fmt.Fprintf(&b, "简介: %s\n", outline.Synopsis)
		if chars := FormatCharacters(outline); chars != "" {
			b.WriteString("\n人物:\n")
			b.WriteString(chars)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// ParseStyleBibleJSON parses and validates style bible output.
func ParseStyleBibleJSON(raw string) (*StyleBible, error) {
	raw = strings.TrimSpace(raw)
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end <= start {
		return nil, fmt.Errorf("style bible: no JSON object")
	}
	var bible StyleBible
	if err := json.Unmarshal([]byte(raw[start:end+1]), &bible); err != nil {
		return nil, fmt.Errorf("style bible JSON: %w", err)
	}
	if strings.TrimSpace(bible.Title) == "" {
		return nil, fmt.Errorf("style bible missing title")
	}
	if len(bible.Protagonists) == 0 {
		return nil, fmt.Errorf("style bible missing protagonists")
	}
	return &bible, nil
}

// LoadStyleBible reads style_bible.json from workflow workspace.
func LoadStyleBible(wf *agentflowiov1alpha1.Workflow) (*StyleBible, error) {
	raw, err := ReadArtifact(wf, StyleBibleArtifact)
	if err != nil {
		return nil, err
	}
	return ParseStyleBibleJSON(raw)
}

// FormatStyleBibleBlock renders bible for chapter worker prompts.
func FormatStyleBibleBlock(bible *StyleBible) string {
	if bible == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("【设定圣经 — 全书必须遵守】\n")
	fmt.Fprintf(&b, "书名: %s\n", bible.Title)
	if bible.POV != "" {
		fmt.Fprintf(&b, "人称: %s\n", bible.POV)
	}
	if bible.Tone != "" {
		fmt.Fprintf(&b, "基调: %s\n", bible.Tone)
	}
	if bible.ProseStyle != "" {
		fmt.Fprintf(&b, "文笔: %s\n", bible.ProseStyle)
	}
	if len(bible.Protagonists) > 0 {
		fmt.Fprintf(&b, "主角（姓名不可更改）: %s\n", strings.Join(bible.Protagonists, "、"))
	}
	if len(bible.SupportingCast) > 0 {
		fmt.Fprintf(&b, "重要配角: %s\n", strings.Join(bible.SupportingCast, "、"))
	}
	if bible.TimelineAnchor != "" {
		fmt.Fprintf(&b, "时空锚点: %s\n", bible.TimelineAnchor)
	}
	if len(bible.Forbidden) > 0 {
		fmt.Fprintf(&b, "严禁: %s\n", strings.Join(bible.Forbidden, "；"))
	}
	return b.String()
}

// ExtractBibleProtagonists parses protagonist names embedded in chapter instructions.
func ExtractBibleProtagonists(instruction string) []string {
	for _, line := range strings.Split(instruction, "\n") {
		line = strings.TrimSpace(line)
		// Must be the bible protagonist label, not incidental 主角 in POV/forbidden lines.
		if !strings.HasPrefix(line, "主角") {
			continue
		}
		if idx := strings.Index(line, ":"); idx >= 0 {
			line = line[idx+1:]
		} else if idx := strings.Index(line, "："); idx >= 0 {
			line = line[idx+1:]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var names []string
		for _, part := range strings.FieldsFunc(line, func(r rune) bool {
			return r == '、' || r == ',' || r == '，' || r == '/'
		}) {
			part = strings.TrimSpace(part)
			if part != "" {
				names = append(names, part)
			}
		}
		if len(names) > 0 {
			return names
		}
	}
	return nil
}

// ProtagonistNamesFromInstruction collects protagonist names from bible block and character lists.
func ProtagonistNamesFromInstruction(instruction string) []string {
	if names := ExtractBibleProtagonists(instruction); len(names) > 0 {
		return names
	}
	return ExtractListedCharacterNames(instruction)
}

// ProtagonistNames returns primary character names from bible or outline.
func ProtagonistNames(bible *StyleBible, outline *NovelOutline) []string {
	if bible != nil && len(bible.Protagonists) > 0 {
		return bible.Protagonists
	}
	if outline == nil {
		return nil
	}
	var names []string
	for _, ch := range outline.Characters {
		if name, _ := ch["name"].(string); name != "" {
			names = append(names, name)
			if len(names) >= 3 {
				break
			}
		}
	}
	return names
}