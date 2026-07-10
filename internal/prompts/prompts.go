// Package prompts centralizes all LLM prompt templates and builders used
// across Planner, Worker, Monitor, and Workflow steps.
//
// Goals:
// - Single place to audit and evolve prompts
// - Easier A/B testing and versioning
// - Consistent formatting and safety instructions
package prompts

import (
	"fmt"
	"strings"
)

// TaskType constants (shared with flow package to avoid import cycles for now).
const (
	TaskTypeNovelOutline            = "novel-outline"
	TaskTypeNovelOutlineRefine      = "novel-outline-refine"
	TaskTypeNovelPlot               = "novel-plot"
	TaskTypeNovelChapter            = "novel-chapter"
	TaskTypeNovelChapterTeam        = "novel-chapter-team"
	TaskTypeNovelOutlineSkeleton    = "novel-outline-skeleton"
)

// GetWorkerSystemPrompt returns the system prompt for the Worker role
// based on the detected task type.
func GetWorkerSystemPrompt(instruction, monitorTaskType string) string {
	taskType := monitorTaskType
	if taskType == "" {
		taskType = detectTaskType(instruction)
	}

	switch taskType {
	case TaskTypeNovelOutline:
		return `你是小说策划编辑。根据指令生成小说大纲，严格只输出一个 JSON 对象。
要求：
1. 不要输出思考过程、分析、英文备注或 markdown 代码块
2. JSON 必须含 title、synopsis、characters、chapters 字段且可被解析
3. chapters 数组每项含 num、title、summary，num 从 1 连续递增
4. 直接以 { 开头、以 } 结尾`

	case TaskTypeNovelOutlineRefine:
		return `你是资深小说策划编辑。根据指令对现有大纲进行精修，严格只输出一个 JSON 对象。
要求：
1. 不要输出思考过程、分析、英文备注或 markdown 代码块
2. 读取 outline.json 改进，保留好的部分；outline-draft.json 为初稿备份仅供对照
3. JSON 必须含 title、synopsis、characters、chapters 字段且可被解析
4. 重点改进：主线完整性、冲突递进、人物弧光、节奏收束、伏笔回收；不得删减或合并章节
5. 直接以 { 开头、以 } 结尾`

	case TaskTypeNovelPlot:
		return `你是小说剧情编剧。根据梗概扩写剧情脚本，只输出剧情脚本文本。
要求：
1. 不要输出思考过程或 markdown 代码块
2. 包含场景节拍、冲突、对话要点、衔接与悬念
3. 不要写成完整散文正文`

	case TaskTypeNovelOutlineSkeleton:
		return `你是小说策划编辑。根据指令生成长篇分卷骨架，严格只输出一个 JSON 对象。
要求：
1. 不要输出思考过程、分析、英文备注或 markdown 代码块
2. JSON 必须含 title、synopsis、characters、volumes 字段且可被解析
3. volumes 每项含 num、title、startChapter、endChapter、theme、summary，章节范围连续无遗漏
4. 直接以 { 开头、以 } 结尾`

	case TaskTypeNovelChapter, TaskTypeNovelChapterTeam:
		target := parseTargetWords(instruction) // lightweight parse
		minRunes := minChapterRunes(target)
		lengthHint := ""
		if target > 0 {
			lengthHint = fmt.Sprintf("5. 本章正文不少于 %d 字（目标约 %d 字），写完整场景与对话，不要草草收尾\n6. 必须以句号、问号或感叹号等完整收束，不要中途截断", minRunes, target)
		} else {
			lengthHint = "5. 正文需充实完整，以完整句子收束，不要中途截断"
		}
		return fmt.Sprintf(`你是小说作者。根据大纲与上下文撰写本章正文。
要求：
1. 不要输出思考过程或写作说明
2. 直接输出中文小说正文，自然衔接上一章
3. 人物、时间线、伏笔与设定保持一致；严禁更换主角姓名或引入未在大纲登记的主要角色
4. 全章保持统一叙事人称、语体与节奏；若分多段撰写，段与段须无缝衔接，不得像拼凑的独立片段
%s`, lengthHint)

	default:
		return "你是一个专业的任务执行者。根据给定的指令执行任务，生成高质量、完整的产出物。\n1. 内容丰富、有深度\n2. 格式规范、排版清晰\n3. 直接输出最终结果，不要输出过程说明"
	}
}

// GetMonitorSystemPrompt returns the system prompt for the Monitor role.
func GetMonitorSystemPrompt(taskType string, threshold int, configPrompt string) string {
	if configPrompt != "" {
		return configPrompt
	}

	base := `你是资深小说质量编辑。严格只输出 JSON：{"score": <0-100整数>, "passed": <bool>, "feedback": "<中文具体可执行建议>", "check_method": "rule|ai|hybrid"}`

	switch taskType {
	case TaskTypeNovelChapter, TaskTypeNovelChapterTeam:
		return base + fmt.Sprintf(`

评估维度（每项 0-20 分，总分 100）：
- 完整性：是否写完整场景、对话、情绪闭环（目标字数意识）
- 人物一致：姓名、性格、动机与大纲/前文一致
- 剧情推进：有冲突/悬念/转折，不平铺直叙
- 文风契合：与 style_bible / 之前章节语气统一
- 细节与沉浸：感官、动作、心理活动自然

通过标准：score >= %d
feedback 必须具体指出问题 + 如何修改（不要空泛表扬）。
直接输出 JSON，不要任何额外文字。`, threshold)

	case TaskTypeNovelOutline, TaskTypeNovelOutlineRefine:
		return base + `

评估大纲质量：
- 主线是否清晰完整
- 人物弧光与冲突递进
- 章节节奏是否合理
- 设定一致性

直接输出 JSON。`

	default:
		return base + "\n\n直接输出 JSON 评估结果。"
	}
}

// Helper (duplicated lightly to avoid cycle; real impl lives in chapter_length)
func parseTargetWords(instruction string) int {
	// Simplified; in real code we call the real parser.
	if strings.Contains(instruction, "2500") {
		return 2500
	}
	return 2000
}

func minChapterRunes(target int) int {
	if target <= 0 {
		return 1200
	}
	return int(float64(target) * 0.9)
}

func detectTaskType(instruction string) string {
	lower := strings.ToLower(instruction)
	switch {
	case strings.Contains(lower, "大纲") && strings.Contains(lower, "精修"):
		return TaskTypeNovelOutlineRefine
	case strings.Contains(lower, "大纲") || strings.Contains(lower, "outline"):
		return TaskTypeNovelOutline
	case strings.Contains(lower, "剧情") || strings.Contains(lower, "plot"):
		return TaskTypeNovelPlot
	case strings.Contains(lower, "正文") || strings.Contains(lower, "chapter"):
		return TaskTypeNovelChapter
	default:
		return ""
	}
}

// --- Migrated prompt builders from workflow package ---

// BuildStyleBibleInstruction builds the prompt for generating the style bible (canon).
func BuildStyleBibleInstruction(prompt string, outlineTitle, outlineSynopsis string, characters string) string {
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

	if outlineTitle != "" || outlineSynopsis != "" {
		b.WriteString("\n\n【已生成大纲】\n")
		if outlineTitle != "" {
			fmt.Fprintf(&b, "书名: %s\n", outlineTitle)
		}
		if outlineSynopsis != "" {
			fmt.Fprintf(&b, "简介: %s\n", outlineSynopsis)
		}
		if characters != "" {
			b.WriteString("\n人物:\n")
			b.WriteString(characters)
			b.WriteString("\n")
		}
	}
	return b.String()
}

// BuildHistoricalResearchInstruction builds the instruction for historical research step.
func BuildHistoricalResearchInstruction(prompt string, era string, workspacePath string, outputDir string) string {
	eraLine := ""
	if era != "" {
		eraLine = fmt.Sprintf("时代背景锚点: %s\n", era)
	}
	return fmt.Sprintf(`%s

你是历史小说调研编辑。必须使用 MCP 工具联网检索，整理可写入小说的史实与民俗资料。

工作区: %s
输出文件: %s/research_notes.md

调研清单（用 historical_research、web_search、wikipedia_search、web_fetch）:
1. 时代与地理：朝代纪年、都城/地域风貌、社会制度
2. 真实人物：身份、关系、可查证的言行典故（标注姓名与称谓）
3. 民俗日常：服饰、饮食、居所、交通、节庆、称谓礼仪
4. 小说边界：哪些可虚构，哪些不得明显违背史料
5. 关键事件与制度：与主角身份相关的朝政、科举、军事、礼法等

输出要求：
- 必须真实可查，标注来源
- 用 bullet points + 小标题组织
- 每条控制在 1-3 句话
- 最后给出「可用于小说虚构的边界建议」

%s`, prompt, workspacePath, outputDir, eraLine)
}

// BuildRewriteInstruction returns the instruction for rewriting a chapter or plot.
func BuildRewriteInstruction(originalPrompt string, chapterNum int, layer string, instruction string) (string, error) {
	if layer == "" {
		layer = "chapter"
	}
	base := fmt.Sprintf(`你是一位专业小说%s编辑。

原指令: %s

请根据用户新指令重写%s #%d 的内容。

用户指令: %s

要求：
1. 保持与大纲、人物、时间线、伏笔的一致性
2. 不要改变主要剧情走向，除非指令明确要求
3. 输出格式与原步骤一致（剧情用脚本格式，正文用小说正文）
4. 直接输出重写后的内容，不要解释`, mapLayer(layer), originalPrompt, mapLayer(layer), chapterNum, instruction)

	return base, nil
}

func mapLayer(l string) string {
	if l == "plot" {
		return "剧情"
	}
	return "章节"
}

// BuildOutlineSyncInstruction builds prompt to sync outline after rewrite.
func BuildOutlineSyncInstruction(chapterNum int) string {
	return fmt.Sprintf(`请根据本次重写的章节 #%d 内容，更新 outline.json 中对应章节的 summary。
只输出 JSON 片段或完整更新后的 chapters 数组相关部分。
保持其他章节不变。`, chapterNum)
}

// BuildArcSummaryInstruction builds instruction for arc summary step.
func BuildArcSummaryInstruction(prompt string, start, end, width int) string {
	return fmt.Sprintf(`%s

你是小说弧光总结编辑。请为章节 %d-%d 生成弧光总结（约 %d 字）。

要求：
- 总结本弧的主要冲突推进、人物变化、关键转折
- 突出伏笔与呼应
- 为后续章节提供清晰上下文
- 直接输出总结文本`, prompt, start, end, width)
}

// BuildSegmentInstruction builds prompt for one segment of a chapter.
func BuildSegmentInstruction(baseInstruction string, segmentIndex, totalSegments, segmentWords int, priorTail, openingSample string) string {
	return fmt.Sprintf(`%s

这是分段写作，第 %d/%d 段。
本段目标字数约 %d 字。
上一段结尾: %s
本段应自然衔接上一段，并为下一段留悬念或过渡。

%s

直接继续写本段正文。`, baseInstruction, segmentIndex, totalSegments, segmentWords, priorTail, openingSample)
}

// BuildRAGContextBlock returns a RAG context block to inject into prompts.
func BuildRAGContextBlock(title, query string, hits []string) string {
	if len(hits) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n【相关剧情参考（RAG）】\n")
	fmt.Fprintf(&b, "查询: %s\n", query)
	for i, h := range hits {
		fmt.Fprintf(&b, "%d. %s\n", i+1, truncate(h, 300))
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "..."
}

// BuildConsistencyMonitorContext returns context for consistency check.
func BuildConsistencyMonitorContext(stepID string) string {
	return fmt.Sprintf("当前步骤ID: %s。请检查与前文人物、时间线、伏笔的一致性。", stepID)
}

// BuildConsistencyAnchor builds a short anchor for continuity.
func BuildConsistencyAnchor(instruction string) string {
	return fmt.Sprintf("前文关键信息摘要（用于保持一致性）：\n%s", instruction)
}

// Additional simple context builders can be added here.

// --- More migrated builders ---

// BuildChapterInstruction renders the prompt for writing a chapter (flattened version).
func BuildChapterInstruction(
	base string,
	title string,
	synopsis string,
	characters string,
	chapterNum int,
	chapterTitle string,
	chapterSummary string,
	plotScript string,
	ragBlock string,
	arcSummaries string,
	storySoFar string,
	recentSummaries string,
	previousEnding string,
	wordsPerChapter int,
	styleBibleBlock string,
) string {
	var b strings.Builder
	if styleBibleBlock != "" {
		b.WriteString(styleBibleBlock)
		b.WriteString("\n\n")
	}
	b.WriteString(base)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "书名: %s\n", title)
	fmt.Fprintf(&b, "全书简介: %s\n", synopsis)
	if characters != "" {
		b.WriteString("\n主要人物（必须保持一致）:\n")
		b.WriteString(characters)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n当前章节: 第%d章《%s》\n", chapterNum, chapterTitle)
	fmt.Fprintf(&b, "本章梗概: %s\n", chapterSummary)
	if plotScript != "" {
		b.WriteString("\n本章剧情脚本（据此写正文，不得偏离）:\n")
		b.WriteString(plotScript)
		b.WriteString("\n")
	}
	if ragBlock != "" {
		b.WriteString("\n")
		b.WriteString(ragBlock)
		b.WriteString("\n")
	}
	if arcSummaries != "" {
		b.WriteString("\n已完成故事弧摘要:\n")
		b.WriteString(arcSummaries)
		b.WriteString("\n")
	}
	if storySoFar != "" {
		b.WriteString("\n更早剧情概要:\n")
		b.WriteString(storySoFar)
		b.WriteString("\n")
	}
	if recentSummaries != "" {
		b.WriteString("\n近几章摘要:\n")
		b.WriteString(recentSummaries)
		b.WriteString("\n")
	}
	if previousEnding != "" {
		b.WriteString("\n上一章结尾（必须自然衔接）:\n")
		b.WriteString(previousEnding)
		b.WriteString("\n")
	}
	if wordsPerChapter > 0 {
		fmt.Fprintf(&b, "\n目标字数约: %d 字（正文不少于 %d 字，须完整收束）\n", wordsPerChapter, minChapterLength(wordsPerChapter))
	}
	b.WriteString("\n要求: 人物性格、时间线、伏笔与上文一致；本章需推进剧情并留下合理悬念；写足篇幅，不要草草收尾。")
	return b.String()
}

func minChapterLength(words int) int {
	return int(float64(words) * 0.9)
}

// BuildPlotInstruction renders plot-stage worker prompt (flattened).
func BuildPlotInstruction(
	base string,
	title string,
	synopsis string,
	characters string,
	chapterNum int,
	chapterTitle string,
	chapterSummary string,
	recentSummaries string,
	previousEnding string,
	ragBlock string,
	styleBibleBlock string,
) string {
	var b strings.Builder
	if styleBibleBlock != "" {
		b.WriteString(styleBibleBlock)
		b.WriteString("\n\n")
	}
	b.WriteString(base)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "书名: %s\n", title)
	fmt.Fprintf(&b, "全书简介: %s\n", synopsis)
	if characters != "" {
		b.WriteString("\n主要人物:\n")
		b.WriteString(characters)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n当前章节: 第%d章《%s》\n", chapterNum, chapterTitle)
	fmt.Fprintf(&b, "本章梗概（据此扩写剧情）: %s\n", chapterSummary)
	if recentSummaries != "" {
		b.WriteString("\n近几章摘要:\n")
		b.WriteString(recentSummaries)
		b.WriteString("\n")
	}
	if previousEnding != "" {
		b.WriteString("\n上一章结尾（剧情须衔接）:\n")
		b.WriteString(previousEnding)
		b.WriteString("\n")
	}
	if ragBlock != "" {
		b.WriteString("\n")
		b.WriteString(ragBlock)
		b.WriteString("\n")
	}
	return b.String()
}

// BuildVolumeOutlineInstruction renders prompt for volume outline.
func BuildVolumeOutlineInstruction(
	prompt string,
	skeletonTitle string,
	skeletonSynopsis string,
	characters string,
	volNum int,
	volTitle string,
	startChapter int,
	endChapter int,
	theme string,
	summary string,
	prevVolumeSummary string,
) string {
	var b strings.Builder
	b.WriteString(prompt)
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "书名: %s\n全书简介: %s\n", skeletonTitle, skeletonSynopsis)
	if characters != "" {
		b.WriteString("\n主要人物:\n")
		b.WriteString(characters)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n当前分卷: 第%d卷《%s》\n", volNum, volTitle)
	fmt.Fprintf(&b, "章节范围: 第%d章 到 第%d章\n", startChapter, endChapter)
	fmt.Fprintf(&b, "本卷主题: %s\n", theme)
	fmt.Fprintf(&b, "本卷概要: %s\n", summary)
	if prevVolumeSummary != "" {
		b.WriteString("\n上一卷章节梗概（保持衔接）:\n")
		b.WriteString(prevVolumeSummary)
		b.WriteString("\n")
	}
	b.WriteString(`
请输出本卷详细章节大纲，严格 JSON（不要 markdown 代码块）：
{
  "volume": 1,
  "chapters": [{"num":1,"title":"章节标题","summary":"章节梗概"}]
}
要求：
1. chapters 必须覆盖本卷全部章节，num 连续
2. 每章 summary 写清冲突、转折及与前后章衔接
3. 人物动机与世界观与前文一致`)
	return b.String()
}

// BuildChapterContext assembles rolling context for a chapter (returns strings for injection).
func BuildChapterContextStrings(
	arcSummaries string,
	storySoFar string,
	recentSummaries string,
	previousEnding string,
) (string, string, string, string) {
	return arcSummaries, storySoFar, recentSummaries, previousEnding
}

// BuildOutlineRefineMonitorContext provides context for outline refine monitor.
func BuildOutlineRefineMonitorContext() string {
	return "工作区 outline-draft.json 为精修前初稿备份，outline.json 为待改进版本。请重点检查主线完整性、人物一致性和节奏。"
}



