package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"agent-flow/internal/ai"
	wfengine "agent-flow/internal/workflow"
)

const (
	TaskTypePoetry       = "poetry"
	TaskTypeCode         = "code"
	TaskTypeGeneral      = "general"
	TaskTypeNovelChapter       = "novel-chapter"
	TaskTypeNovelChapterTeam   = "novel-chapter-team"
	TaskTypeNovelOutline         = "novel-outline"
	TaskTypeNovelOutlineRefine   = "novel-outline-refine"
	TaskTypeNovelOutlineSkeleton = "novel-outline-skeleton"
	TaskTypeNovelStyleBible      = "novel-style-bible"
	TaskTypeNovelVolumeOutline   = "novel-volume-outline"

	CheckMethodRule   = "rule"
	CheckMethodAI     = "ai"
	CheckMethodHybrid = "hybrid"

	// MonitorTierLight runs a compact AI rubric (consistency + prose).
	MonitorTierLight = "light"
	// MonitorTierFull runs the full editor rubric.
	MonitorTierFull = "full"
)

// EvalDimensions holds per-dimension scores.
type EvalDimensions struct {
	Completeness int `json:"completeness"`
	Accuracy     int `json:"accuracy"`
	Quality      int `json:"quality"`
}

// EvalResult is the structured output from Monitor evaluation.
type EvalResult struct {
	Score       int             `json:"score"`
	Passed      bool            `json:"passed"`
	Feedback    string          `json:"feedback"`
	Issues      []string        `json:"issues,omitempty"`
	Dimensions  *EvalDimensions `json:"dimensions,omitempty"`
	TaskType    string          `json:"taskType"`
	CheckMethod string          `json:"checkMethod"`
	Attempt     int             `json:"attempt"`
}

// DetectTaskType infers evaluation rubric from the worker instruction.
func DetectTaskType(instruction string) string {
	lower := strings.ToLower(instruction)
	poetryKeywords := []string{"诗", "绝句", "律诗", "押韵", "poem", "poetry", "诗词", "七言", "五言"}
	codeKeywords := []string{"代码", "程序", "函数", "code", "script", "python", "golang", "hello world", "实现"}

	for _, kw := range poetryKeywords {
		if strings.Contains(lower, kw) {
			return TaskTypePoetry
		}
	}
	for _, kw := range codeKeywords {
		if strings.Contains(lower, kw) {
			return TaskTypeCode
		}
	}
	if strings.Contains(instruction, "分卷骨架") || strings.Contains(instruction, `"volumes"`) && strings.Contains(instruction, "startChapter") {
		return TaskTypeNovelOutlineSkeleton
	}
	if strings.Contains(instruction, "详细章节大纲") || strings.Contains(instruction, `"volume"`) && strings.Contains(instruction, `"chapters"`) {
		return TaskTypeNovelVolumeOutline
	}
	return TaskTypeGeneral
}

// RunRuleChecks performs fast deterministic validation before AI evaluation.
func RunRuleChecks(instruction, output, taskType string) *EvalResult {
	trimmed := strings.TrimSpace(output)
	result := &EvalResult{
		TaskType:    taskType,
		CheckMethod: CheckMethodRule,
		Dimensions:  &EvalDimensions{},
	}

	if trimmed == "" {
		result.Score = 0
		result.Passed = false
		result.Feedback = "产出物为空，必须生成实际内容"
		result.Issues = []string{"empty_output"}
		return result
	}

	if len([]rune(trimmed)) < 8 {
		result.Score = 15
		result.Passed = false
		result.Feedback = "产出物过短，内容不完整"
		result.Issues = []string{"too_short"}
		return result
	}

	issues := []string{}
	score := 60

	if strings.Contains(trimmed, "```") && !balancedFences(trimmed) {
		issues = append(issues, "unclosed_code_fence")
		score -= 20
	}

	placeholderPatterns := []string{"TODO", "待完成", "placeholder", "未完成", "稍后补充"}
	for _, p := range placeholderPatterns {
		if strings.Contains(strings.ToLower(trimmed), strings.ToLower(p)) {
			issues = append(issues, "contains_placeholder:"+p)
			score -= 15
		}
	}

	switch taskType {
	case TaskTypePoetry:
		score, issues = evaluatePoetryRules(trimmed, issues, score)
	case TaskTypeCode:
		score, issues = evaluateCodeRules(trimmed, issues, score)
	case TaskTypeNovelOutline, TaskTypeNovelOutlineRefine:
		score, issues = evaluateNovelOutlineRules(instruction, trimmed, issues, score)
	case TaskTypeNovelOutlineSkeleton:
		score, issues = evaluateNovelSkeletonRules(instruction, trimmed, issues, score)
	case TaskTypeNovelStyleBible:
		score, issues = evaluateNovelStyleBibleRules(trimmed, issues, score)
	case TaskTypeNovelChapter, TaskTypeNovelChapterTeam:
		score, issues = evaluateNovelChapterRules(instruction, trimmed, issues, score)
		if taskType == TaskTypeNovelChapterTeam {
			score, issues = evaluateTeamChapterRules(instruction, trimmed, issues, score)
		}
	default:
		score, issues = evaluateGeneralRules(instruction, trimmed, issues, score)
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	result.Score = score
	result.Issues = issues
	result.Dimensions.Completeness = min(score, 40)
	result.Dimensions.Accuracy = min(score*3/10, 30)
	result.Dimensions.Quality = min(score*3/10, 30)

	if len(issues) == 0 {
		result.Feedback = "规则预检通过，等待 AI 深度评估"
	} else {
		result.Feedback = fmt.Sprintf("规则预检发现问题: %s", strings.Join(issues, "; "))
	}
	return result
}

func evaluatePoetryRules(output string, issues []string, score int) (int, []string) {
	lines := splitPoetryLines(output)
	if len(lines) < 4 {
		issues = append(issues, "insufficient_lines")
		score -= 25
	}

	sevenCharLines := 0
	for _, line := range lines {
		chars := chineseCharCount(line)
		if chars == 7 {
			sevenCharLines++
		} else if chars > 0 && (chars < 5 || chars > 9) {
			issues = append(issues, fmt.Sprintf("irregular_line_length:%d", chars))
			score -= 5
		}
	}

	if strings.Contains(output, "七言") && sevenCharLines < 4 {
		issues = append(issues, "not_seven_char_quatrain")
		score -= 20
	}

	return score, issues
}

func evaluateCodeRules(output string, issues []string, score int) (int, []string) {
	codeSignals := []string{"func ", "def ", "class ", "import ", "package ", "return ", "{", "}", "();", "println", "printf", "#include"}
	found := 0
	lower := strings.ToLower(output)
	for _, sig := range codeSignals {
		if strings.Contains(lower, strings.ToLower(sig)) {
			found++
		}
	}
	if found == 0 {
		issues = append(issues, "no_code_structure")
		score -= 30
	}
	return score, issues
}

func evaluateNovelOutlineRules(instruction, output string, issues []string, score int) (int, []string) {
	trimmed := strings.TrimSpace(NormalizeWorkerOutput(instruction, output))
	if !strings.HasPrefix(trimmed, "{") {
		issues = append(issues, "not_json_object")
		score -= 40
		return score, issues
	}
	if !strings.Contains(trimmed, `"chapters"`) {
		issues = append(issues, "missing_chapters_field")
		score -= 30
		return score, issues
	}
	if _, err := wfengine.ParseOutlineJSON(trimmed); err != nil {
		issues = append(issues, "invalid_outline_json")
		score -= 35
		return score, issues
	}
	if len([]rune(trimmed)) < 80 {
		issues = append(issues, "outline_too_short")
		score -= 20
		return score, issues
	}
	return 100, issues
}

var skeletonChapterCountRE = regexp.MustCompile(`全书共\s*(\d+)\s*章`)

func evaluateNovelSkeletonRules(instruction, output string, issues []string, score int) (int, []string) {
	trimmed := strings.TrimSpace(NormalizeWorkerOutput(instruction, output))
	if !strings.HasPrefix(trimmed, "{") {
		issues = append(issues, "not_json_object")
		score -= 40
		return score, issues
	}
	if !strings.Contains(trimmed, `"volumes"`) {
		issues = append(issues, "missing_volumes_field")
		score -= 30
		return score, issues
	}
	skeleton, err := wfengine.ParseSkeletonJSON(trimmed)
	if err != nil {
		issues = append(issues, "invalid_skeleton_json")
		score -= 35
		return score, issues
	}
	if skeleton.Title == "" || skeleton.Synopsis == "" {
		issues = append(issues, "missing_title_or_synopsis")
		score -= 15
	}
	expected := 0
	if m := skeletonChapterCountRE.FindStringSubmatch(instruction); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			expected = n
		}
	}
	if len(skeleton.Volumes) == 0 {
		issues = append(issues, "empty_volumes")
		return 0, issues
	}
	prevEnd := 0
	for i, vol := range skeleton.Volumes {
		if vol.StartChapter <= 0 || vol.EndChapter < vol.StartChapter {
			issues = append(issues, fmt.Sprintf("invalid_volume_range:%d", vol.Num))
			score -= 20
		}
		if i == 0 && vol.StartChapter != 1 {
			issues = append(issues, "volume_must_start_at_1")
			score -= 20
		}
		if prevEnd > 0 && vol.StartChapter != prevEnd+1 {
			issues = append(issues, fmt.Sprintf("volume_gap_after_%d", prevEnd))
			score -= 25
		}
		prevEnd = vol.EndChapter
	}
	if expected > 0 && prevEnd != expected {
		issues = append(issues, fmt.Sprintf("chapter_coverage_mismatch:want_%d_got_%d", expected, prevEnd))
		score -= 30
	}
	if len(issues) == 0 {
		return 100, issues
	}
	return score, issues
}

func evaluateGeneralRules(instruction, output string, issues []string, score int) (int, []string) {
	instrRunes := len([]rune(instruction))
	outRunes := len([]rune(output))
	if instrRunes > 20 && outRunes < instrRunes/4 {
		issues = append(issues, "output_too_brief_for_task")
		score -= 15
	}
	return score, issues
}

// BuildMonitorSystemPrompt returns the system prompt for AI evaluation.
func BuildMonitorSystemPrompt(taskType string, threshold int, configPrompt string) string {
	// 配置文件中的 monitor prompt 面向诗词场景；小说/通用任务使用内置 rubric。
	useConfig := strings.TrimSpace(configPrompt) != "" &&
		(taskType == "" || taskType == TaskTypePoetry)
	if useConfig {
		return fmt.Sprintf("%s\n\n通过阈值: %d 分（score >= %d 时 passed 应为 true）。", strings.TrimSpace(configPrompt), threshold, threshold)
	}

	switch taskType {
	case TaskTypeNovelOutline:
		return fmt.Sprintf(`你是小说策划编辑，审查大纲 JSON 产出质量。

评分标准（总分 100）：
- 结构 (30分)：必须含 title、synopsis、chapters 数组，JSON 可解析
- 章节 (30分)：章节数量与要求一致，num 连续，每章有 title/summary
- 故事性 (40分)：人物设定清晰，故事弧完整，章节梗概有冲突与衔接，主线贯穿全书

通过阈值: %d 分

仅返回 JSON：
{"score": 分数, "passed": true/false, "feedback": "评价", "issues": ["问题"], "dimensions": {"completeness": 分, "accuracy": 分, "quality": 分}}`, threshold)
	case TaskTypeNovelStyleBible:
		return fmt.Sprintf(`你是小说总编，审查「设定圣经」是否可作为全书写作契约。

评分标准（总分 100）：
- 完整性 (40分)：含 title、pov、tone、protagonists、forbidden 等关键字段
- 准确性 (35分)：protagonists 与大纲人物一致，禁忌项明确
- 可执行性 (25分)：文笔/人称/节奏规范具体可遵守

通过阈值: %d 分

仅返回 JSON：
{"score": 分数, "passed": true/false, "feedback": "评价", "issues": ["问题"], "dimensions": {"completeness": 分, "accuracy": 分, "quality": 分}}`, threshold)
	case TaskTypeNovelOutlineRefine:
		return fmt.Sprintf(`你是小说策划编辑，审查大纲精修版质量（附加上下文含精修前初稿）。

评分标准（总分 100）：
- 结构 (30分)：必须含 title、synopsis、chapters 数组，JSON 可解析
- 章节 (30分)：章节数量与初稿一致，num 连续，每章有 title/summary，不得删章改号
- 改进度 (40分)：相比初稿，故事弧、人物设定、章节冲突与衔接是否明显改进

通过阈值: %d 分

仅返回 JSON：
{"score": 分数, "passed": true/false, "feedback": "评价", "issues": ["问题"], "dimensions": {"completeness": 分, "accuracy": 分, "quality": 分}}`, threshold)
	case TaskTypeNovelOutlineSkeleton:
		return fmt.Sprintf(`你是小说策划编辑，审查长篇分卷骨架大纲质量。

评分标准（总分 100）：
- 结构 (30分)：含 title、synopsis、characters、volumes 数组，JSON 可解析
- 覆盖 (40分)：volumes 无重叠无遗漏，startChapter/endChapter 连续覆盖全书章节
- 故事性 (30分)：各卷主题递进，人物弧光清晰，末卷预留高潮与收束

通过阈值: %d 分

仅返回 JSON：
{"score": 分数, "passed": true/false, "feedback": "评价", "issues": ["问题"], "dimensions": {"completeness": 分, "accuracy": 分, "quality": 分}}`, threshold)
	case TaskTypeNovelChapterTeam:
		return fmt.Sprintf(`你是长篇小说责编（团队模式终审），审查润色后的章节。

评分标准（总分 100）：
- 设定符合 (30分)：主角姓名、人设与设定圣经一致
- 衔接连贯 (30分)：与上一章结尾自然衔接，无跳剪拼凑感
- 剧情符合 (25分)：落实本章梗概，无严重跑题
- 文笔统一 (15分)：人称语体一致，无元评论

通过阈值: %d 分

仅返回 JSON：
{"score": 分数, "passed": true/false, "feedback": "评价", "issues": ["问题"], "dimensions": {"completeness": 分, "accuracy": 分, "quality": 分}}`, threshold)
	case TaskTypeNovelChapter:
		return fmt.Sprintf(`你是长篇小说编辑，负责审查章节与全书设定、前文的一致性（全量质检）。

评分标准（总分 100）：
- 衔接性 (30分)：与上一章结尾、时间线是否自然衔接，有无突兀跳转
- 人物一致性 (25分)：角色名称、性格、关系是否与设定和前文一致
- 剧情符合度 (25分)：是否落实本章大纲梗概，有无严重偏离或自相矛盾
- 文字质量 (20分)：叙事完整、节奏合理、无明显水文或重复

通过阈值: %d 分

若发现人物改名、时间线冲突、剧情断层、设定矛盾，必须在 issues 中标注（如 character_rename, timeline_conflict, plot_jump）。

仅返回 JSON：
{"score": 分数, "passed": true/false, "feedback": "评价", "issues": ["问题"], "dimensions": {"completeness": 分, "accuracy": 分, "quality": 分}}`, threshold)
	case TaskTypePoetry:
		return fmt.Sprintf(`你是一位古典诗词鉴赏专家，负责审查诗歌产出质量。

评分标准（总分 100）：
- 格律 (40分)：句长、押韵、对仗
- 内容 (40分)：意境、用词、主题契合
- 艺术性 (20分)：流畅度、情感表达

通过阈值: %d 分

仅返回 JSON，不要其他文字：
{"score": 分数, "passed": true/false, "feedback": "评价", "issues": ["问题"], "dimensions": {"completeness": 分, "accuracy": 分, "quality": 分}}`, threshold)
	case TaskTypeCode:
		return fmt.Sprintf(`你是代码审查专家，评估代码产出质量。

评分标准（总分 100）：
- 完整性 (40分)：是否完整可运行
- 正确性 (30分)：逻辑与语法
- 规范性 (30分)：格式与可读性

通过阈值: %d 分

仅返回 JSON：
{"score": 分数, "passed": true/false, "feedback": "评价", "issues": ["问题"], "dimensions": {"completeness": 分, "accuracy": 分, "quality": 分}}`, threshold)
	default:
		return fmt.Sprintf(`你是质量检查监工，评估任务产出质量。

评分标准（总分 100）：
- 完整性 (40分)：是否涵盖任务要求
- 准确性 (30分)：内容是否正确
- 质量 (30分)：表达与格式

通过阈值: %d 分

仅返回 JSON：
{"score": 分数, "passed": true/false, "feedback": "评价", "issues": ["问题"], "dimensions": {"completeness": 分, "accuracy": 分, "quality": 分}}`, threshold)
	}
}

// EvaluateWithAI calls Monitor AI and merges with rule-check hints.
func EvaluateWithAI(ctx context.Context, aiSvc *ai.Service, systemPrompt, instruction, output string, ruleHint *EvalResult, threshold, attempt int) (*EvalResult, error) {
	userMessage := fmt.Sprintf("任务要求：%s\n\n产出物：\n%s", instruction, output)
	if ruleHint != nil && len(ruleHint.Issues) > 0 {
		userMessage += fmt.Sprintf("\n\n规则预检提示（请纳入评估）: %s", strings.Join(ruleHint.Issues, "; "))
	}

	response, err := aiSvc.MonitorChat(ctx, systemPrompt, userMessage)
	if err != nil {
		return nil, err
	}

	eval, err := ParseMonitorResult(response, threshold)
	if err != nil {
		return &EvalResult{
			Score:       30,
			Passed:      false,
			Feedback:    fmt.Sprintf("评估解析失败: %v。原始响应: %s", err, truncateForFeedback(response)),
			CheckMethod: CheckMethodAI,
			Attempt:     attempt,
		}, nil
	}

	eval.CheckMethod = CheckMethodAI
	if ruleHint != nil {
		eval.CheckMethod = CheckMethodHybrid
		eval.TaskType = ruleHint.TaskType
	}
	eval.Attempt = attempt
	return eval, nil
}

func evaluateNovelChapterRules(instruction, output string, issues []string, score int) (int, []string) {
	target := ParseTargetWordsFromInstruction(instruction)
	minLen := MinChapterRunes(target)

	runes := len([]rune(output))
	if runes < minLen {
		issues = append(issues, "chapter_too_short")
		score -= 25
	}
	if LooksTruncated(output) {
		issues = append(issues, "chapter_truncated")
		score -= 30
	}

	metaSection := instruction
	if idx := strings.Index(instruction, "【跨章一致性检查】"); idx != -1 {
		metaSection = instruction[idx:]
	}
	names := wfengine.ProtagonistNamesFromInstruction(metaSection)
	if len(names) == 0 {
		names = wfengine.ExtractListedCharacterNames(metaSection)
	}
	if len(names) > 0 {
		mentioned := 0
		for _, name := range names {
			if strings.Contains(output, name) {
				mentioned++
			}
		}
		if mentioned == 0 {
			issues = append(issues, "no_main_character_mentioned")
			score -= 20
		}
	}

	repeatMarkers := []string{"未完待续", "下一章预告", "作者的话", "写作说明"}
	for _, marker := range repeatMarkers {
		if strings.Contains(output, marker) {
			issues = append(issues, "meta_commentary:"+marker)
			score -= 10
		}
	}
	return score, issues
}

// ResolveMonitorTier picks light vs full monitor for workflow chapters.
func ResolveMonitorTier(taskType string, attempt int, arcBoundary, firstChapter bool) string {
	switch taskType {
	case TaskTypeNovelOutline, TaskTypeNovelOutlineRefine, TaskTypeNovelOutlineSkeleton, TaskTypeNovelVolumeOutline, TaskTypeNovelStyleBible:
		return MonitorTierFull
	case TaskTypeNovelChapter, TaskTypeNovelChapterTeam:
		if attempt >= 1 || arcBoundary || firstChapter {
			return MonitorTierFull
		}
		return MonitorTierLight
	default:
		return MonitorTierFull
	}
}

// BuildLightMonitorSystemPrompt is a compact rubric for per-chapter QC (consistency + prose).
func BuildLightMonitorSystemPrompt(threshold int) string {
	return fmt.Sprintf(`你是长篇小说责编，对本章做轻量质检（一致性 + 可读性并重）。

评分标准（总分 100）：
- 衔接与设定 (35分)：与上一章结尾、人物表、时间线是否自然衔接，有无突兀跳转或设定矛盾
- 剧情符合度 (35分)：是否落实本章梗概要点，有无严重跑题或自相矛盾
- 文笔与节奏 (30分)：叙事完整、对话自然、节奏合理，无明显水文、重复段落或「作者的话」类元评论

通过阈值: %d 分

若发现人物改名、时间线冲突、剧情断层、草草收尾，必须在 issues 中标注。

仅返回 JSON：
{"score": 分数, "passed": true/false, "feedback": "评价", "issues": ["问题"], "dimensions": {"completeness": 分, "accuracy": 分, "quality": 分}}`, threshold)
}

// RunMonitorEvaluation executes the full monitor pipeline.
func RunMonitorEvaluation(ctx context.Context, aiSvc *ai.Service, instruction, output string, threshold, attempt int, configPrompt, previousFeedback, taskType, consistencyContext, tier string, teamMode bool) (*EvalResult, error) {
	if taskType == "" {
		taskType = DetectTaskType(instruction)
	}
	normalizedOutput := output
	if taskType == TaskTypeNovelOutline || taskType == TaskTypeNovelOutlineRefine || taskType == TaskTypeNovelOutlineSkeleton {
		normalizedOutput = NormalizeWorkerOutput(instruction, output)
	}

	ruleResult := RunRuleChecks(instruction, normalizedOutput, taskType)
	ruleResult.Attempt = attempt
	ruleResult.TaskType = taskType

	if ruleResult.Score == 0 {
		return ruleResult, nil
	}

	// 团队章节：L0 规则全通过则直接放行（执笔者+润色已完成，避免 Monitor AI 误杀/超时）。
	if teamMode && taskType == TaskTypeNovelChapterTeam && len(ruleResult.Issues) == 0 {
		if ruleResult.Score < 85 {
			ruleResult.Score = 85
		}
		ruleResult.Passed = true
		ruleResult.CheckMethod = CheckMethodRule
		ruleResult.Feedback = "团队章节 L0 规则质检通过（设定/衔接/篇幅/主角）"
		return ruleResult, nil
	}

	// 设定圣经：规则通过即可放行（JSON 结构校验已足够）。章节轻量/全量质检必须走 AI（一致性+文笔）。
	if len(ruleResult.Issues) == 0 && ruleResult.Score >= threshold {
		if taskType == TaskTypeNovelStyleBible {
			if _, err := wfengine.ParseStyleBibleJSON(normalizedOutput); err == nil {
				ruleResult.Passed = true
				ruleResult.CheckMethod = CheckMethodRule
				ruleResult.Feedback = "设定圣经 JSON 结构校验通过"
				return ruleResult, nil
			}
		}
	}

	userInstruction := instruction
	if consistencyContext != "" {
		userInstruction = consistencyContext + "\n\n---\n\n写作指令:\n" + instruction
	}
	if previousFeedback != "" {
		userInstruction += "\n\n上次评估反馈（请在改进中体现）: " + previousFeedback
	}

	if teamMode && taskType == TaskTypeNovelChapterTeam && ruleResult.Score > 0 && len(ruleResult.Issues) > 0 {
		if canon, err := evaluateCanonGate(ctx, aiSvc, userInstruction, normalizedOutput, threshold, attempt); err == nil && canon != nil && !canon.Passed {
			canon.TaskType = taskType
			return canon, nil
		}
	}

	systemPrompt := BuildMonitorSystemPrompt(taskType, threshold, configPrompt)
	if (taskType == TaskTypeNovelChapter || taskType == TaskTypeNovelChapterTeam) && tier == MonitorTierLight {
		systemPrompt = BuildLightMonitorSystemPrompt(threshold)
	}
	aiResult, err := EvaluateWithAI(ctx, aiSvc, systemPrompt, userInstruction, normalizedOutput, ruleResult, threshold, attempt)
	if err != nil {
		return nil, err
	}

	if aiResult.TaskType == "" {
		aiResult.TaskType = taskType
	}

	// Rule failures can cap the final score when issues are severe.
	if len(ruleResult.Issues) > 0 && aiResult.Score > ruleResult.Score+10 {
		aiResult.Score = (aiResult.Score + ruleResult.Score) / 2
		aiResult.Passed = aiResult.Score >= threshold
		aiResult.Issues = mergeIssues(ruleResult.Issues, aiResult.Issues)
		if aiResult.Feedback != "" {
			aiResult.Feedback = ruleResult.Feedback + "\n" + aiResult.Feedback
		}
	}

	return aiResult, nil
}

// FormatRetryFeedback builds actionable feedback for the next Worker attempt.
func FormatRetryFeedback(eval *EvalResult, threshold int) string {
	if eval == nil {
		return "质量检查未通过，请改进产出"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "质量评分 %d/%d（未通过）。", eval.Score, threshold)
	if fb := sanitizeMonitorFeedback(eval.Feedback, eval.Issues); fb != "" {
		fmt.Fprintf(&b, "\n评价: %s", fb)
	}
	if len(eval.Issues) > 0 {
		fmt.Fprintf(&b, "\n需修复: %s", strings.Join(eval.Issues, "; "))
	}
	if eval.Dimensions != nil {
		fmt.Fprintf(&b, "\n分项: 完整性=%d, 准确性=%d, 质量=%d",
			eval.Dimensions.Completeness, eval.Dimensions.Accuracy, eval.Dimensions.Quality)
	}
	return b.String()
}

// ParseMonitorResult parses Monitor AI JSON response.
func ParseMonitorResult(response string, threshold int) (*EvalResult, error) {
	response = strings.TrimSpace(response)
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end <= start {
		return parseMonitorResultFallback(response, threshold), nil
	}

	jsonStr := response[start : end+1]
	var raw struct {
		Score      int             `json:"score"`
		Passed     *bool           `json:"passed"`
		Pass       *bool           `json:"pass"`
		Feedback   string          `json:"feedback"`
		Issues     []string        `json:"issues"`
		Dimensions *EvalDimensions `json:"dimensions"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return parseMonitorResultFallback(response, threshold), nil
	}

	score := clampScore(raw.Score)
	passed := score >= threshold
	if raw.Passed != nil {
		passed = *raw.Passed
	} else if raw.Pass != nil {
		passed = *raw.Pass
	}

	return &EvalResult{
		Score:      score,
		Passed:     passed,
		Feedback:   sanitizeMonitorFeedback(raw.Feedback, raw.Issues),
		Issues:     raw.Issues,
		Dimensions: raw.Dimensions,
	}, nil
}

func parseMonitorResultFallback(response string, threshold int) *EvalResult {
	scorePatterns := []string{"\"score\":", "score:", "Score:", "评分:", "得分:"}
	for _, pattern := range scorePatterns {
		idx := strings.Index(response, pattern)
		if idx == -1 {
			continue
		}
		rest := strings.TrimSpace(response[idx+len(pattern):])
		numStr := ""
		for _, c := range rest {
			if c >= '0' && c <= '9' {
				numStr += string(c)
			} else if numStr != "" {
				break
			}
		}
		if numStr != "" {
			score := 0
			for _, c := range numStr {
				score = score*10 + int(c-'0')
			}
			score = clampScore(score)
			return &EvalResult{
				Score:    score,
				Passed:   score >= threshold,
				Feedback: sanitizeMonitorFeedback(response, nil),
			}
		}
	}

	return &EvalResult{
		Score:    30,
		Passed:   false,
		Feedback: "Monitor AI 未返回标准评分格式，无法评估质量",
	}
}

var jsonFeedbackField = regexp.MustCompile(`"feedback"\s*:\s*"((?:\\.|[^"\\])*)"`)

func sanitizeMonitorFeedback(raw string, issues []string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return issueGuidanceFromTags(issues)
	}
	if fb := extractJSONFeedbackField(raw); fb != "" {
		return truncateRunes(fb, 480)
	}
	if line := extractChineseEvalLine(raw); line != "" {
		return truncateRunes(line, 480)
	}
	if len([]rune(raw)) > 320 || looksLikeChainOfThought(raw) {
		if hint := issueGuidanceFromTags(issues); hint != "" {
			return hint
		}
		return "质检未通过：请严格按大纲人物、剧情与上文衔接重写，全章保持统一文风，不得拼凑互不相干的片段。"
	}
	return truncateRunes(raw, 480)
}

func extractJSONFeedbackField(raw string) string {
	if m := jsonFeedbackField.FindStringSubmatch(raw); len(m) == 2 {
		var decoded string
		if err := json.Unmarshal([]byte(`"`+m[1]+`"`), &decoded); err == nil {
			return strings.TrimSpace(decoded)
		}
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractChineseEvalLine(raw string) string {
	keywords := []string{"严重偏离", "主角姓名", "剧情完全", "无法通过", "不得矛盾", "必须衔接", "人物改名", "设定矛盾"}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if chineseCharCount(line) < 12 {
			continue
		}
		for _, kw := range keywords {
			if strings.Contains(line, kw) {
				line = strings.Trim(line, `"`)
				return line
			}
		}
	}
	return ""
}

func looksLikeChainOfThought(raw string) bool {
	lower := strings.ToLower(raw)
	markers := []string{"let's ", "wait,", "i will ", "self-correction", "okay,", "final plan", "checking the"}
	hits := 0
	for _, m := range markers {
		if strings.Contains(lower, m) {
			hits++
		}
	}
	return hits >= 2
}

func issueGuidanceFromTags(issues []string) string {
	if len(issues) == 0 {
		return ""
	}
	var hints []string
	for _, issue := range issues {
		switch issue {
		case "no_main_character_mentioned", "character_rename", "missing_characters":
			hints = append(hints, "必须使用大纲登记的主角姓名，不得擅自改名或替换主角")
		case "plot_jump", "plot_deviation":
			hints = append(hints, "剧情须符合本章梗概，不得跑题或跳剪")
		case "chapter_truncated", "chapter_too_short":
			hints = append(hints, "写足篇幅并以完整句子收束")
		case "timeline_conflict", "setting_conflict":
			hints = append(hints, "时间线与设定须与上文一致")
		default:
			hints = append(hints, issue)
		}
	}
	return strings.Join(uniqueStrings(hints), "；")
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func splitPoetryLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "===") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func chineseCharCount(s string) int {
	count := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			count++
		}
	}
	return count
}

func balancedFences(s string) bool {
	return strings.Count(s, "```")%2 == 0
}

func mergeIssues(a, b []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, list := range [][]string{a, b} {
		for _, item := range list {
			if item == "" || seen[item] {
				continue
			}
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func truncateForFeedback(s string) string {
	if len(s) <= 200 {
		return s
	}
	return s[:200] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
