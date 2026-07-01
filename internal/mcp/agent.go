package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Agent struct {
	mcpURL     string
	maxSteps   int
	httpClient *http.Client
}

type AgentConfig struct {
	MCPURL   string
	MaxSteps int
}

func NewAgent(cfg AgentConfig) *Agent {
	if cfg.MaxSteps <= 0 {
		cfg.MaxSteps = 10
	}
	return &Agent{
		mcpURL:     strings.TrimRight(cfg.MCPURL, "/"),
		maxSteps:   cfg.MaxSteps,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

type AgentResult struct {
	Output string
	Steps  []AgentStep
	Error  error
}

type AgentStep struct {
	Thought  string
	Tool     string
	ToolArgs map[string]interface{}
	ToolOut  string
}

// FetchToolCatalog loads tool descriptions from MCP /tools/list.
func (a *Agent) FetchToolCatalog(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.mcpURL+"/tools/list", nil)
	if err != nil {
		return "", err
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", err
	}
	var result struct {
		Tools []ToolInfo `json:"tools"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if len(result.Tools) == 0 {
		return "", fmt.Errorf("no tools returned from MCP server")
	}

	tools := make([]Tool, 0, len(result.Tools))
	for _, info := range result.Tools {
		tools = append(tools, staticTool{name: info.Name, desc: info.Description})
	}
	return FormatToolCatalog(tools), nil
}

type staticTool struct {
	name string
	desc string
}

func (t staticTool) Name() string        { return t.name }
func (t staticTool) Description() string { return t.desc }
func (t staticTool) Execute(context.Context, map[string]interface{}) (string, error) {
	return "", fmt.Errorf("not executable")
}

func (a *Agent) Run(ctx context.Context, task string, toolDescriptions string) (*AgentResult, error) {
	systemPrompt := fmt.Sprintf(`你是一个任务执行 Agent。你可以使用以下工具来完成任务：

%s

执行任务时，请按以下格式思考和行动：

Thought: 分析当前情况，决定下一步行动
Action: 工具名称
ActionInput: {"参数名": "参数值"}

当你完成任务时，输出最终结果：
Thought: 任务已完成
FinalAnswer: 最终输出内容

重要规则：
1. 每次只执行一个工具调用
2. 如果不需要工具，直接给出 FinalAnswer
3. 工具执行结果会作为 Observation 返回给你
4. 最多执行 %d 步`, toolDescriptions, a.maxSteps)

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

		resp, err := a.chat(ctx, messages)
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
			messages = append(messages, ChatMessage{Role: "user", Content: "请按照格式输出 Action 和 ActionInput，或者给出 FinalAnswer。"})
			continue
		}

		steps[len(steps)-1].Tool = toolName
		steps[len(steps)-1].ToolArgs = toolInput

		toolResult, err := a.callTool(ctx, toolName, toolInput)
		if err != nil {
			toolResult = fmt.Sprintf("工具调用失败: %v", err)
		}
		steps[len(steps)-1].ToolOut = toolResult

		messages = append(messages, ChatMessage{Role: "assistant", Content: resp})
		messages = append(messages, ChatMessage{Role: "user", Content: fmt.Sprintf("Observation: %s\n\n请继续思考下一步行动，或给出 FinalAnswer。", toolResult)})
	}

	return &AgentResult{
		Output: steps[len(steps)-1].Thought,
		Steps:  steps,
		Error:  fmt.Errorf("reached max steps (%d) without FinalAnswer", a.maxSteps),
	}, nil
}

func (a *Agent) chat(ctx context.Context, messages []ChatMessage) (string, error) {
	body := ChatRequest{Messages: messages}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequestWithContext(ctx, "POST", a.mcpURL+"/ai/chat", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	var result ChatResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response failed: %w, body: %s", err, string(respBody[:min(len(respBody), 500)]))
	}
	if result.Error != "" {
		return "", fmt.Errorf("AI error: %s", result.Error)
	}
	return result.Content, nil
}

func (a *Agent) callTool(ctx context.Context, name string, input map[string]interface{}) (string, error) {
	reqBody := ToolCallRequest{Name: name, Input: input}
	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequestWithContext(ctx, "POST", a.mcpURL+"/tools/call", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	var result ToolCallResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse tool response failed: %w", err)
	}
	if result.Error != "" {
		return result.Output, fmt.Errorf("%s", result.Error)
	}
	return result.Output, nil
}

func parseAction(text string) (string, map[string]interface{}, bool) {
	actionIdx := strings.Index(text, "Action:")
	if actionIdx == -1 {
		return "", nil, false
	}
	actionLine := strings.TrimSpace(text[actionIdx+len("Action:"):])
	actionLine = strings.SplitN(actionLine, "\n", 2)[0]
	actionLine = strings.TrimSpace(actionLine)

	inputIdx := strings.Index(text, "ActionInput:")
	if inputIdx == -1 {
		return actionLine, map[string]interface{}{}, true
	}
	inputStr := strings.TrimSpace(text[inputIdx+len("ActionInput:"):])
	inputStr = strings.SplitN(inputStr, "\n", 2)[0]
	inputStr = strings.TrimSpace(inputStr)

	var input map[string]interface{}
	if err := json.Unmarshal([]byte(inputStr), &input); err != nil {
		return actionLine, map[string]interface{}{}, true
	}
	return actionLine, input, true
}
