package flow

import (
	"context"
	"fmt"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PolishNode unifies prose style after draft generation (team pipeline).
type PolishNode struct {
	Name string
}

// Run polishes WorkerOutput in place without changing plot or character names.
func (n *PolishNode) Run(ctx context.Context, input State) (State, error) {
	input.Phase = "polishing"
	logger := log.FromContext(ctx).WithName("polish-node")

	if !input.TeamMode || input.MonitorTaskType != TaskTypeNovelChapterTeam {
		return input, nil
	}
	if input.AIService == nil {
		return input, fmt.Errorf("AI 服务未初始化")
	}
	draft := strings.TrimSpace(input.WorkerOutput)
	if draft == "" {
		return input, fmt.Errorf("润色输入为空")
	}

	systemPrompt := `你是长篇小说的润色编辑（Line Editor）。输入为执笔者初稿。
任务：在不改变剧情走向、人物姓名与关键事实的前提下润色正文。
要求：
1. 统一人称、时态、语体与叙事节奏，消除段间拼凑感
2. 加强段落与章节内衔接，删除重复句、元评论、提纲句、作者腔
3. 不得更换或新增主角姓名，不得引入未在大纲出现的主要角色
4. 保留场景、对话与心理描写，可压缩冗余但不可大幅删情节
只输出润色后的中文小说正文，不要标题重复堆砌，不要写作说明。`

	userMessage := fmt.Sprintf("【写作约束】\n%s\n\n【初稿】\n%s", input.WorkerInstruction, draft)
	result, err := input.AIService.WorkerChat(ctx, systemPrompt, userMessage)
	if err != nil {
		return input, fmt.Errorf("润色 AI 调用失败：%w", err)
	}
	input.TokenUsage.Add(result.Usage)

	polished := ExtractChineseProse(strings.TrimSpace(result.Content))
	if len([]rune(polished)) < len([]rune(draft))/3 {
		logger.Info("polish output too short, keeping draft", "draft", len(draft), "polished", len(polished))
		return input, nil
	}

	input.WorkerOutput = polished
	input.ExecutionResult = fmt.Sprintf("润色完成，长度 %d → %d 字符", len(draft), len(polished))
	logger.Info("chapter polished", "before", len(draft), "after", len(polished))
	return input, nil
}