package workflow

import (
	"encoding/json"
	"fmt"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
	"agent-flow/internal/prompts"
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
	chars := ""
	if outline != nil {
		chars = FormatCharacters(outline)
	}
	title := ""
	syn := ""
	if outline != nil {
		title = outline.Title
		syn = outline.Synopsis
	}
	return prompts.BuildStyleBibleInstruction(prompt, title, syn, chars)
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
