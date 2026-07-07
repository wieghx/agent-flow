package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"agent-flow/internal/config"
)

// OllamaClient Ollama API 客户端
type OllamaClient struct {
	baseURL     string
	model       string
	temperature float64
	topP        float64
	maxTokens   int
	timeout     time.Duration
	httpClient  *http.Client
}

// OllamaChatMessage 聊天消息
type OllamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaChatRequest 聊天请求
type OllamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []OllamaChatMessage `json:"messages"`
	Options  OllamaOptions       `json:"options"`
	Stream   bool                `json:"stream"`
}

// OllamaOptions 选项
type OllamaOptions struct {
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	NumPredict  int     `json:"num_predict"`
}

// OllamaChatResponse 聊天响应
type OllamaChatResponse struct {
	Model            string `json:"model"`
	CreatedAt        string `json:"created_at"`
	Message          struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done             bool `json:"done"`
	PromptEvalCount  int  `json:"prompt_eval_count"`
	EvalCount        int  `json:"eval_count"`
}

// NewOllamaClient 创建 Ollama 客户端
func NewOllamaClient(localConfig *config.LocalConfig) *OllamaClient {
	return &OllamaClient{
		baseURL:     localConfig.BaseURL,
		model:       localConfig.Model,
		temperature: localConfig.Temperature,
		topP:        localConfig.TopP,
		maxTokens:   localConfig.MaxTokens,
		timeout:     localConfig.GetTimeout(),
		httpClient: &http.Client{
			Timeout: localConfig.GetTimeout(),
		},
	}
}

// Chat 发送聊天请求
func (c *OllamaClient) Chat(ctx context.Context, systemPrompt, userMessage string) (ChatResult, error) {
	url := c.baseURL + "/api/chat"

	request := OllamaChatRequest{
		Model:  c.model,
		Stream: false,
		Options: OllamaOptions{
			Temperature: c.temperature,
			TopP:        c.topP,
			NumPredict:  c.maxTokens,
		},
		Messages: []OllamaChatMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userMessage,
			},
		},
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return ChatResult{}, fmt.Errorf("序列化请求失败：%w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return ChatResult{}, fmt.Errorf("创建请求失败：%w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ChatResult{}, fmt.Errorf("发送请求失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ChatResult{}, fmt.Errorf("API 返回错误状态：%d, 响应：%s", resp.StatusCode, string(body))
	}

	var ollamaResp OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&ollamaResp); err != nil {
		return ChatResult{}, fmt.Errorf("解析响应失败：%w", err)
	}

	usage := TokenUsage{
		PromptTokens:     ollamaResp.PromptEvalCount,
		CompletionTokens: ollamaResp.EvalCount,
		TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
	}
	return ChatResult{Content: ollamaResp.Message.Content, Usage: usage}, nil
}

// Generate 生成文本（非聊天模式）
func (c *OllamaClient) Generate(ctx context.Context, prompt string) (string, error) {
	url := c.baseURL + "/api/generate"

	request := map[string]interface{}{
		"model":  c.model,
		"prompt": prompt,
		"options": map[string]interface{}{
			"temperature": c.temperature,
			"top_p":       c.topP,
			"num_predict": c.maxTokens,
		},
		"stream": false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败：%w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 返回错误状态：%d, 响应：%s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("解析响应失败：%w", err)
	}

	content, ok := result["response"].(string)
	if !ok {
		return "", fmt.Errorf("响应中没有 response 字段")
	}

	return content, nil
}

// Check 检查 Ollama 服务是否可用
func (c *OllamaClient) Check(ctx context.Context) error {
	url := c.baseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("无法连接到 Ollama 服务：%w", err)
	}
	defer resp.Body.Close()

	return nil
}

// ListModels 列出可用模型
func (c *OllamaClient) ListModels(ctx context.Context) ([]string, error) {
	url := c.baseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接到 Ollama 服务：%w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := []string{}
	modelList, ok := result["models"].([]interface{})
	if !ok {
		return models, nil
	}

	for _, m := range modelList {
		if modelMap, ok := m.(map[string]interface{}); ok {
			if name, ok := modelMap["name"].(string); ok {
				models = append(models, name)
			}
		}
	}

	return models, nil
}

// GetModel 实现 AIService.GetModel
func (c *OllamaClient) GetModel() string {
	return c.model
}
