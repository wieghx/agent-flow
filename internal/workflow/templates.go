package workflow

import (
	"fmt"
	"strconv"
	"strings"

	agentflowiov1alpha1 "agent-flow/api/v1alpha1"
)

// BuildSpecFromTemplate materializes workflow steps from a named template.
func BuildSpecFromTemplate(template string, prompt string, params map[string]string) (agentflowiov1alpha1.WorkflowSpec, error) {
	switch template {
	case "", "novel-outline-chapters", "novel-team-chapters", "novel-team-historical":
		return novelProductionSpec(prompt, params, template), nil
	case "novel-import-deconstruct":
		return importDeconstructSpec(prompt, params), nil
	case "novel-chapter-rewrite":
		return chapterRewriteSpec(prompt, params), nil
	default:
		return agentflowiov1alpha1.WorkflowSpec{}, fmt.Errorf("unknown workflow template: %s", template)
	}
}

func novelProductionSpec(prompt string, params map[string]string, template string) agentflowiov1alpha1.WorkflowSpec {
	team := TeamModeEnabled(params)
	if template == "novel-team-historical" {
		team = true
		if params == nil {
			params = map[string]string{}
		}
		params["historicalResearch"] = "true"
		template = "novel-team-chapters"
	}
	if template == "" || template == "novel-outline-chapters" {
		if team {
			template = "novel-team-chapters"
		} else {
			template = "novel-outline-chapters"
		}
	}
	research := HistoricalResearchEnabled(params, prompt)
	chapterCount := IntParam(params, "chapterCount", 10)
	wordsPerChapter := IntParam(params, "wordsPerChapter", 3000)
	quality := int32(IntParam(params, "qualityThreshold", DefaultQualityThreshold))

	execution := agentflowiov1alpha1.WorkflowExecution{
		Mode:            ExecutionModeParallel,
		MaxParallel:     DefaultMaxParallel,
		StepMaxRetries:  DefaultStepMaxRetries,
		PauseOnStepFail: DefaultPauseOnStepFail,
		AutoApprove:     true,
	}
	if v := IntParam(params, "maxParallel", 0); v > 0 {
		execution.MaxParallel = v
	}
	if v := IntParam(params, "stepMaxRetries", 0); v > 0 {
		execution.StepMaxRetries = v
	}
	if v := IntParam(params, "stepRetryBaseDelaySec", 0); v > 0 {
		execution.StepRetryBaseDelaySec = v
	}
	if v := IntParam(params, "stepRetryMaxDelaySec", 0); v > 0 {
		execution.StepRetryMaxDelaySec = v
	}
	execution.PauseOnStepFail = BoolParam(params, "pauseOnStepFail", execution.PauseOnStepFail)
	if UseVolumeOutline(params, chapterCount) {
		execution.ChapterMode = ChapterModePipeline
		execution.ChapterPipeline = DefaultChapterPipeline
		if v := IntParam(params, "chapterPipeline", 0); v > 0 {
			execution.ChapterPipeline = v
		}
	}

	spec := agentflowiov1alpha1.WorkflowSpec{
		Prompt:    prompt,
		Template:  template,
		Params:    params,
		Execution: execution,
		Workspace: agentflowiov1alpha1.WorkflowWorkspace{
			PVC:      "task-outputs",
			BasePath: "",
		},
	}

	var chapterInstruction string
	if team {
		chapterInstruction = teamChapterForeachInstruction(wordsPerChapter, params)
	} else {
		segCount := SegmentCount(params, wordsPerChapter)
		segWords := SegmentWordsPerPart(params, wordsPerChapter)
		chapterInstruction = fmt.Sprintf(`你是小说作者。根据大纲、人物设定、故事弧摘要、近几章摘要和上一章结尾撰写本章正文。
目标字数约: %d 字（正文不少于 %d 字，须以完整句子收束）。
必须与前文自然衔接，人物姓名、性格、时间线与伏笔保持一致；写足场景、对话与心理描写，不要草草收尾。
严禁更换主角或引入未在大纲登记的主要角色；全章须文风统一、情节连贯，不得写成互不相干的片段拼凑。`, wordsPerChapter, MinChapterLength(wordsPerChapter))
		if SegmentModeEnabled(params, wordsPerChapter) {
			chapterInstruction += fmt.Sprintf(`
系统将分 %d 段生成后拼接为一章；你每次只需写其中一段，段与段必须语体、人称与叙事节奏完全一致。`, segCount)
			chapterInstruction = AppendSegmentDirectives(chapterInstruction, segCount, segWords)
		}
		chapterInstruction += `
只输出章节正文（可含章节标题），不要解释写作过程。`
	}

	outlineDep := "outline-refine"
	if BoolParam(params, "importedNovel", false) {
		outlineDep = "import-rag-index"
	}
	threeStage := ThreeStageEnabled(params)

	var plotStep agentflowiov1alpha1.WorkflowStep
	if threeStage {
		plotStep = agentflowiov1alpha1.WorkflowStep{
			ID:        "plots",
			Name:      "逐章扩写剧情",
			Type:      agentflowiov1alpha1.WorkflowStepTypeForeach,
			DependsOn: plotForeachDependsOn(outlineDep),
			Foreach: &agentflowiov1alpha1.WorkflowForeach{
				Source:       "outline.json",
				JSONPath:     "chapters",
				StepIDPrefix: "plot",
				OutputPath:   "chapters/chapter-{{num}}.plot.md",
				Instruction:  plotForeachInstruction(params),
			},
			TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
				QualityThreshold: quality,
				MonitorTaskType:  "novel-plot",
			},
		}
	}

	chaptersStep := agentflowiov1alpha1.WorkflowStep{
		ID:        "chapters",
		Name:      "逐章生成正文",
		Type:      agentflowiov1alpha1.WorkflowStepTypeForeach,
		DependsOn: proseForeachDependsOn(team, threeStage),
		Foreach: &agentflowiov1alpha1.WorkflowForeach{
			Source:       "outline.json",
			JSONPath:     "chapters",
			StepIDPrefix: "chapter",
			OutputPath:   "chapters/chapter-{{num}}.md",
			Instruction:  chapterInstruction,
		},
		TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
			QualityThreshold: quality,
			MonitorTaskType:  novelChapterMonitorType(team),
		},
	}

	mergeStep := agentflowiov1alpha1.WorkflowStep{
		ID:        "merge",
		Name:      "合并书稿",
		Type:      agentflowiov1alpha1.WorkflowStepTypeMerge,
		DependsOn: []string{"chapters"},
		TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
			WorkerInstruction: `读取工作区 chapters 目录下所有 chapter-*.md，按章节号排序，
生成完整书稿并输出 Markdown（含书名、简介、目录、各章正文）。`,
			MCPMode:          true,
			QualityThreshold: 70,
		},
		Output: agentflowiov1alpha1.WorkflowStepOutput{Path: "book.md", Format: "markdown"},
	}

	if UseVolumeOutline(params, chapterCount) {
		volSteps := buildVolumeOutlineSteps(prompt, params, chapterCount, research)
		if research {
			spec.Steps = append(spec.Steps, historicalResearchStep(quality))
			if len(volSteps) > 0 {
				volSteps[0].DependsOn = []string{"historical-research"}
			}
		}
		spec.Steps = append(spec.Steps, volSteps...)
		spec.Steps = append(spec.Steps, outlineRefineStep(prompt, chapterCount, quality, "outline-merge"))
		if team {
			spec.Steps = append(spec.Steps, styleBibleStep(prompt, []string{"outline-refine"}, quality))
		}
	} else if !BoolParam(params, "importedNovel", false) {
		if research {
			spec.Steps = append(spec.Steps, historicalResearchStep(quality))
		}
		outlineDeps := []string{}
		if research {
			outlineDeps = append(outlineDeps, "historical-research")
		}
		spec.Steps = append(spec.Steps, simpleOutlineStepWithResearch(prompt, chapterCount, research))
		if len(outlineDeps) > 0 {
			spec.Steps[len(spec.Steps)-1].DependsOn = outlineDeps
		}
		spec.Steps = append(spec.Steps, outlineRefineStep(prompt, chapterCount, quality, "outline"))
		if team {
			spec.Steps = append(spec.Steps, styleBibleStep(prompt, []string{"outline-refine"}, quality))
		}
	} else if team {
		spec.Steps = append(spec.Steps, styleBibleStep(prompt, []string{outlineDep}, quality))
	}

	if threeStage {
		spec.Steps = append(spec.Steps, plotStep)
	}
	spec.Steps = append(spec.Steps, chaptersStep, mergeStep)
	return spec
}

func novelChapterMonitorType(team bool) string {
	if team {
		return "novel-chapter-team"
	}
	return "novel-chapter"
}

func simpleOutlineStep(prompt string, chapterCount int) agentflowiov1alpha1.WorkflowStep {
	return simpleOutlineStepWithResearch(prompt, chapterCount, false)
}

func simpleOutlineStepWithResearch(prompt string, chapterCount int, research bool) agentflowiov1alpha1.WorkflowStep {
	researchHint := ""
	if research {
		researchHint = `
若工作区已有 research_notes.md，须将其中真实历史人物、时代制度与民俗细节融入大纲；人物身份称谓须符合时代，不得与史料常识明显矛盾。`
	}
	return agentflowiov1alpha1.WorkflowStep{
		ID:   "outline",
		Name: "生成小说大纲",
		Type: agentflowiov1alpha1.WorkflowStepTypeAITask,
		TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
			WorkerInstruction: fmt.Sprintf(`%s

请生成一部小说的完整大纲，严格输出 JSON（不要 markdown 代码块）：
{
  "title": "书名",
  "synopsis": "全书简介",
  "characters": [{"name":"角色名","role":"定位","trait":"特征"}],
  "chapters": [{"num":1,"title":"章节标题","summary":"章节梗概"}]
}
要求：
1. chapters 数组必须恰好包含 %d 章，num 从 1 连续递增到 %d
2. 每章 summary 写清冲突、转折与与前后章衔接点
3. 全书分为若干故事弧（开端/发展/高潮/收束），长篇需保持主线清晰
4. 题材突出生存与冲突，人物动机一致%s`, prompt, chapterCount, chapterCount, researchHint),
			QualityThreshold: 70,
			MonitorTaskType:  "novel-outline",
		},
		Output: agentflowiov1alpha1.WorkflowStepOutput{Path: "outline.json", Format: "json"},
	}
}

func outlineRefineStep(prompt string, chapterCount int, quality int32, dependsOn ...string) agentflowiov1alpha1.WorkflowStep {
	if len(dependsOn) == 0 {
		dependsOn = []string{"outline"}
	}
	return agentflowiov1alpha1.WorkflowStep{
		ID:        "outline-refine",
		Name:      "大纲精修",
		Type:      agentflowiov1alpha1.WorkflowStepTypeAITask,
		DependsOn: dependsOn,
		TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
			WorkerInstruction: fmt.Sprintf(`%s

读取工作区 outline.json，审查并改进大纲：
1. 检查故事弧是否完整（开端→发展→高潮→收束）
2. 检查人物设定是否清晰，characters 与全书主线一致
3. 检查每章 summary 是否有冲突、转折与前后章衔接
4. chapters 必须恰好 %d 章，num 从 1 连续递增到 %d，不得删减或合并章节
保留好的部分，只改进薄弱环节。

严格输出 JSON（不要 markdown 代码块），格式与原大纲一致：
{"title":"","synopsis":"","characters":[],"chapters":[{"num":1,"title":"","summary":""}]}`, prompt, chapterCount, chapterCount),
			QualityThreshold: quality,
			MonitorTaskType:  "novel-outline-refine",
		},
		Output: agentflowiov1alpha1.WorkflowStepOutput{Path: "outline.json", Format: "json"},
	}
}

func buildVolumeOutlineSteps(prompt string, params map[string]string, chapterCount int, research bool) []agentflowiov1alpha1.WorkflowStep {
	volumeSize := VolumeSize(params)
	volumes := PlanVolumes(chapterCount, volumeSize)
	researchHint := ""
	if research {
		researchHint = `
若工作区已有 research_notes.md，分卷骨架须融入真实历史人物、时代制度；第100章须预留主线收束空间，禁止草草收尾。`
	}

	steps := []agentflowiov1alpha1.WorkflowStep{
		{
			ID:   "outline-skeleton",
			Name: "生成分卷骨架大纲",
			Type: agentflowiov1alpha1.WorkflowStepTypeAITask,
			TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
				WorkerInstruction: fmt.Sprintf(`%s

请生成长篇小说的分卷骨架，严格输出 JSON（不要 markdown 代码块）：
{
  "title": "书名",
  "synopsis": "全书简介",
  "characters": [{"name":"角色名","role":"定位","trait":"特征"}],
  "volumes": [
    {"num":1,"title":"第一卷名","startChapter":1,"endChapter":%d,"theme":"本卷主题","summary":"本卷概要"}
  ]
}
要求：
1. 全书共 %d 章，分为 %d 卷，每卷约 %d 章
2. volumes 覆盖 1-%d 章，无重叠无遗漏
3. 各卷主线递进，人物弧光清晰，最后一卷完成高潮与收束
4. 文笔导向：章节梗概须推进情节，禁止灌水凑字%s`, prompt, volumes[0].End, chapterCount, len(volumes), volumeSize, chapterCount, researchHint),
				QualityThreshold: 70,
				MonitorTaskType:  "novel-outline-skeleton",
			},
			Output: agentflowiov1alpha1.WorkflowStepOutput{Path: "skeleton.json", Format: "json"},
		},
	}

	for _, vol := range volumes {
		steps = append(steps, agentflowiov1alpha1.WorkflowStep{
			ID:        VolumeStepID(vol.Num),
			Name:      fmt.Sprintf("第%d卷章节大纲（%d-%d章）", vol.Num, vol.Start, vol.End),
			Type:      agentflowiov1alpha1.WorkflowStepTypeAITask,
			DependsOn: []string{"outline-skeleton"},
			TaskSpec: agentflowiov1alpha1.WorkflowTaskSpec{
				WorkerInstruction: fmt.Sprintf(`%s

读取工作区 skeleton.json，为第%d卷（第%d-%d章）生成详细章节大纲。
输出 JSON（不要 markdown 代码块）：
{"volume":%d,"chapters":[{"num":%d,"title":"章节标题","summary":"章节梗概"}]}
要求：chapters 恰好覆盖第%d到第%d章，num 连续，每章 summary 写清冲突与衔接。`,
					prompt, vol.Num, vol.Start, vol.End, vol.Num, vol.Start, vol.Start, vol.End),
				QualityThreshold: 70,
				MonitorTaskType:  "novel-volume-outline",
			},
			Output: agentflowiov1alpha1.WorkflowStepOutput{Path: VolumeFileName(vol.Num), Format: "json"},
		})
	}

	volDeps := make([]string, len(volumes))
	for i, vol := range volumes {
		volDeps[i] = VolumeStepID(vol.Num)
	}
	steps = append(steps, agentflowiov1alpha1.WorkflowStep{
		ID:        "outline-merge",
		Name:      "合并分卷大纲",
		Type:      agentflowiov1alpha1.WorkflowStepTypeMerge,
		DependsOn: volDeps,
		Output:    agentflowiov1alpha1.WorkflowStepOutput{Path: "outline.json", Format: "json"},
	})
	return steps
}

// IntParam reads an integer workflow parameter.
func IntParam(params map[string]string, key string, fallback int) int {
	if params == nil {
		return fallback
	}
	raw := strings.TrimSpace(params[key])
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}
