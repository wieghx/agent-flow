package mcp

import (
	"context"
	"fmt"
	"strings"
)

// ChatFunc performs one LLM turn (system + user).
type ChatFunc func(ctx context.Context, systemPrompt, userMessage string) (string, error)

// ToolExecutor runs MCP tools in-process (no HTTP sidecar).
type ToolExecutor struct {
	tools map[string]Tool
}

func NewToolExecutor(toolList []Tool) *ToolExecutor {
	m := make(map[string]Tool, len(toolList))
	for _, t := range toolList {
		m[t.Name()] = t
	}
	return &ToolExecutor{tools: m}
}

func (e *ToolExecutor) Call(ctx context.Context, name string, input map[string]interface{}) (string, error) {
	tool, ok := e.tools[name]
	if !ok {
		return "", fmt.Errorf("tool not found: %s", name)
	}
	return tool.Execute(ctx, input)
}

// LocalAgent runs ReAct loop with in-process tools (for AGENTFLOW_SKIP_SANDBOX).
type LocalAgent struct {
	chat     ChatFunc
	executor *ToolExecutor
	maxSteps int
}

type LocalAgentConfig struct {
	Chat     ChatFunc
	Executor *ToolExecutor
	MaxSteps int
}

func NewLocalAgent(cfg LocalAgentConfig) *LocalAgent {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 12
	}
	if cfg.Executor == nil {
		cfg.Executor = NewToolExecutor(DefaultTools())
	}
	return &LocalAgent{chat: cfg.Chat, executor: cfg.Executor, maxSteps: cfg.MaxSteps}
}

func (a *LocalAgent) Run(ctx context.Context, task, toolDescriptions string) (*AgentResult, error) {
	if a.chat == nil {
		return nil, fmt.Errorf("chat function not configured")
	}
	systemPrompt := fmt.Sprintf(`你是一个任务执行 Agent，可使用工具联网调研并写入工作区文件。

%s

格式：
Thought: ...
Action: 工具名
ActionInput: {"key": "value"}

完成时：
Thought: 任务已完成
FinalAnswer: 最终 Markdown 正文（将写入工作区）

规则：
1. 每次只调用一个工具
2. 历史调研优先用 historical_research / web_search / wikipedia_search，必要时 web_fetch 深挖
3. 结果用 file_write 保存到指令指定路径
4. 最多 %d 步`, toolDescriptions, a.maxSteps)

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: task},
	}

	var steps []AgentStep
	for step := 0; step < a.maxSteps; step++ {
		select {
		case <-ctx.Done():
			return &AgentResult{Steps: steps, Error: ctx.Err()}, nil
		default:
		}

		resp, err := a.chatTurn(ctx, messages)
		if err != nil {
			return &AgentResult{Steps: steps, Error: fmt.Errorf("AI chat failed at step %d: %w", step, err)}, nil
		}
		steps = append(steps, AgentStep{Thought: resp})

		if idx := strings.Index(resp, "FinalAnswer:"); idx != -1 {
			finalAnswer := strings.TrimSpace(resp[idx+len("FinalAnswer:"):])
			return &AgentResult{Output: finalAnswer, Steps: steps}, nil
		}

		toolName, toolInput, ok := parseAction(resp)
		if !ok {
			messages = append(messages, ChatMessage{Role: "assistant", Content: resp})
			messages = append(messages, ChatMessage{Role: "user", Content: "请按 Action/ActionInput 格式调用工具，或给出 FinalAnswer。"})
			continue
		}

		steps[len(steps)-1].Tool = toolName
		steps[len(steps)-1].ToolArgs = toolInput

		toolResult, err := a.executor.Call(ctx, toolName, toolInput)
		if err != nil {
			toolResult = fmt.Sprintf("工具调用失败: %v", err)
		}
		steps[len(steps)-1].ToolOut = toolResult

		messages = append(messages, ChatMessage{Role: "assistant", Content: resp})
		messages = append(messages, ChatMessage{Role: "user", Content: fmt.Sprintf("Observation: %s\n\n请继续，或给出 FinalAnswer。", toolResult)})
	}

	return &AgentResult{
		Output: steps[len(steps)-1].Thought,
		Steps:  steps,
		Error:  fmt.Errorf("reached max steps (%d) without FinalAnswer", a.maxSteps),
	}, nil
}

func (a *LocalAgent) chatTurn(ctx context.Context, messages []ChatMessage) (string, error) {
	if len(messages) == 0 {
		return "", fmt.Errorf("empty messages")
	}
	system := messages[0].Content
	var b strings.Builder
	for _, m := range messages[1:] {
		fmt.Fprintf(&b, "[%s]\n%s\n\n", m.Role, m.Content)
	}
	return a.chat(ctx, system, strings.TrimSpace(b.String()))
}
