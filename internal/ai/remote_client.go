package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"agent-flow/internal/config"
	applog "agent-flow/internal/log"
)

// RemoteClient 远程 AI API 客户端
type RemoteClient struct {
	baseURL     string
	apiKey      string
	model       string
	temperature float64
	maxTokens   int
	timeout     time.Duration
	httpClient  *http.Client
	requestCfg  config.RequestConfig
	responseCfg config.ResponseConfig
}

// RemoteChatMessage 远程聊天消息
type RemoteChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewRemoteClient 创建远程 API 客户端
func NewRemoteClient(remoteConfig *config.RemoteConfig) *RemoteClient {
	return &RemoteClient{
		baseURL:     remoteConfig.BaseURL,
		apiKey:      remoteConfig.APIKey,
		model:       remoteConfig.Model,
		temperature: remoteConfig.Temperature,
		maxTokens:   remoteConfig.MaxTokens,
		timeout:     remoteConfig.GetTimeout(),
		httpClient: &http.Client{
			Timeout: remoteConfig.GetTimeout(),
		},
		requestCfg:  remoteConfig.Request,
		responseCfg: remoteConfig.Response,
	}
}

// Chat 发送聊天请求
func (c *RemoteClient) Chat(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	url := c.baseURL + "/v1/chat/completions"

	var jsonData []byte
	var err error

	// 如果配置了模板，使用模板生成请求
	if c.requestCfg.BodyTemplate != "" {
		jsonData, err = c.generateRequestFromTemplate(systemPrompt, userMessage)
		if err != nil {
			return "", fmt.Errorf("生成请求失败：%w", err)
		}
	} else {
		// 使用默认格式
		jsonData, err = c.generateDefaultRequest(systemPrompt, userMessage)
		if err != nil {
			return "", fmt.Errorf("生成请求失败：%w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败：%w", err)
	}

	// 添加请求头
	for key, value := range c.requestCfg.Headers {
		// 替换 ${AI_API_KEY} 环境变量
		value = strings.ReplaceAll(value, "${AI_API_KEY}", c.apiKey)
		req.Header.Set(key, value)
	}

	// 添加 Authorization 头
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 返回错误状态：%d, 响应：%s", resp.StatusCode, string(body))
	}

	// 解析响应
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("解析响应失败：%w", err)
	}

	// 首先尝试标准 content 字段（API 返回的实际响应内容）
	content := c.extractField(response, c.responseCfg.ExtractField)

	// 如果 content 字段是空的，尝试直接使用 choices[0].message.content
	if content == "" {
		content = c.extractField(response, "choices[0].message.content")
	}

	// 如果还是空的，尝试从 reasoning 字段提取结构化内容（诗歌/JSON）
	// 不回退到原始 reasoning 文本，只提取有用内容
	if content == "" {
		reasoning := c.extractField(response, "choices[0].message.reasoning")
		if reasoning != "" {
			content = c.extractContentFromReasoning(reasoning)
		}
	}

	if content == "" {
		return "", fmt.Errorf("响应中没有有效的内容")
	}

	return content, nil
}

// extractContentFromReasoning 从 reasoning 字段提取实际内容
// vLLM/Qwen 模型在 reasoning 中包含中文诗歌行（7 个字符的纯中文行）或 JSON 格式
func (c *RemoteClient) extractContentFromReasoning(reasoning string) string {
	// 首先尝试从 reasoning 中提取 JSON
	// 查找第一个 { 和最后一个 } 之间的内容
	start := strings.Index(reasoning, "{")
	end := strings.LastIndex(reasoning, "}")
	novelJSON := strings.Contains(reasoning, `"chapters"`) || strings.Contains(reasoning, `"title"`)
	if start != -1 && end != -1 && end > start {
		jsonCandidate := strings.TrimSpace(reasoning[start : end+1])
		jsonCandidate = strings.TrimRight(jsonCandidate, "`")
		jsonCandidate = strings.TrimLeft(jsonCandidate, "`")
		var js json.RawMessage
		if json.Unmarshal([]byte(jsonCandidate), &js) == nil {
			return jsonCandidate
		}
		// 小说大纲类 reasoning 常含合法 JSON 但带杂质；优先返回 JSON 片段而非诗歌行
		if novelJSON && len(jsonCandidate) > 40 {
			return jsonCandidate
		}
	}

	// 大纲/JSON 任务：无合法 JSON 时不返回 reasoning 噪声，触发上层重试
	if novelJSON {
		return ""
	}

	// 如果没找到 JSON，尝试查找诗歌
	lines := strings.Split(reasoning, "\n")
	var poemLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 从每一行中查找连续的 7-10 个中文字符（仅用于诗歌生成）
		chineseSeq := findChineseSequence(line)
		if chineseSeq == "" {
			continue
		}
		runeCount := len([]rune(chineseSeq))
		// 过滤掉平仄格律模式（如：平平仄仄仄平平）
		if isTonalPattern(chineseSeq) {
			continue
		}
		if runeCount >= 7 && runeCount <= 10 {
			poemLines = append(poemLines, chineseSeq)
		}
	}

	// 仅当诗歌片段足够长时才视为诗歌产出（避免把 reasoning 里的短句误判为章节正文）
	if len(poemLines) >= 4 {
		if len(poemLines) > 4 {
			poemLines = poemLines[len(poemLines)-4:]
		}
		result := strings.Join(poemLines, "\n")
		if len([]rune(result)) >= 100 {
			applog.Component("remote-client").Debug("extracted poem lines from reasoning", "lines", len(poemLines))
			return result
		}
	}

	// 如果诗歌行数不足，直接返回 reasoning 的后半部分（跳过思考过程）
	if len(reasoning) > 200 {
		// 尝试找到诗歌开始的位置
		for i, line := range lines {
			line = strings.TrimSpace(line)
			if len([]rune(line)) >= 7 {
				// 检查是否全是中文字符
				allChinese := true
				for _, r := range line {
					if r < 0x4E00 || r > 0x9FFF {
						allChinese = false
						break
					}
				}
				if allChinese {
					// 找到诗歌开始位置，返回剩余内容
					remaining := strings.Join(lines[i:], "\n")
					if len(remaining) > 10 {
						applog.Component("remote-client").Debug("extracted poem fragment from reasoning")
						return remaining
					}
				}
			}
		}
		// 如果没找到诗歌，返回 reasoning 的后半部分
		half := len(reasoning) / 2
		return reasoning[half:]
	}

	return ""
}

// findChineseSequence 查找字符串中第一个 7-10 个连续的中文诗句
func findChineseSequence(s string) string {
	var seq strings.Builder
	var runeCount int

	for _, r := range s {
		if (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF) {
			seq.WriteRune(r)
			runeCount++
			// 如果达到 7 个中文字符，检查是否以标点结尾或长度合适
			if runeCount >= 7 {
				// 检查是否以标点结尾
				lastChar := []rune(seq.String())[runeCount-1]
				if lastChar == '，' || lastChar == '。' || lastChar == '！' || lastChar == '？' {
					// 包含标点，返回整个序列（最多 10 个字符）
					if runeCount <= 10 {
						return seq.String()
					}
				} else if runeCount >= 7 && runeCount <= 10 {
					// 没有标点但长度合适，返回
					return seq.String()
				}
			}
		} else {
			// 非中文字符，重置序列
			seq.Reset()
			runeCount = 0
		}
	}

	return ""
}

// isTonalPattern 判断是否是平仄格律模式（如：平平仄仄仄平平）
func isTonalPattern(s string) bool {
	runes := []rune(s)
	if len(runes) != 7 {
		return false
	}
	// 只包含平/仄字符
	for _, r := range runes {
		if r != '平' && r != '仄' {
			return false
		}
	}
	return true
}

// generateDefaultRequest 生成默认格式的请求
func (c *RemoteClient) generateDefaultRequest(systemPrompt, userMessage string) ([]byte, error) {
	data := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		},
		"temperature": c.temperature,
		"max_tokens":  c.maxTokens,
	}

	return json.Marshal(data)
}

// generateRequestFromTemplate 使用模板生成请求
func (c *RemoteClient) generateRequestFromTemplate(systemPrompt, userMessage string) ([]byte, error) {
	// 直接构建 JSON，避免模板转义问题
	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userMessage},
	}
	data := map[string]interface{}{
		"model":       c.model,
		"messages":    messages,
		"temperature": c.temperature,
		"max_tokens":  c.maxTokens,
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %w", err)
	}

	// 替换 ${AI_API_KEY} 环境变量
	result := string(jsonData)
	result = strings.ReplaceAll(result, "${AI_API_KEY}", c.apiKey)

	return []byte(result), nil
}

// extractField 从响应中提取字段（支持 JSON Path）
// 支持路径格式：choices[0].message.content 或 choices[0].message.reasoning
func (c *RemoteClient) extractField(data map[string]interface{}, path string) string {
	if path == "" {
		return ""
	}

	// 解析路径
	fields := c.parseJSONPath(path)
	return c.extractFromFields(data, fields, 0)
}

// parseJSONPath 解析 JSON Path，将 choices[0].message.content 转换为 ["choices", "0", "message", "content"]
func (c *RemoteClient) parseJSONPath(path string) []string {
	var fields []string
	var currentField strings.Builder

	for i := 0; i < len(path); i++ {
		ch := path[i]
		if ch == '.' {
			if currentField.Len() > 0 {
				fields = append(fields, currentField.String())
				currentField.Reset()
			}
		} else if ch == '[' {
			if currentField.Len() > 0 {
				fields = append(fields, currentField.String())
				currentField.Reset()
			}
			// 查找匹配的 ]
			j := i + 1
			for j < len(path) && path[j] != ']' {
				j++
			}
			if j < len(path) {
				idx := path[i+1 : j]
				fields = append(fields, idx)
				i = j
			}
		} else {
			currentField.WriteByte(ch)
		}
	}

	if currentField.Len() > 0 {
		fields = append(fields, currentField.String())
	}

	return fields
}

// extractFromFields 递归地从字段列表中提取值
func (c *RemoteClient) extractFromFields(current interface{}, fields []string, idx int) string {
	if idx >= len(fields) {
		// 没有更多字段，返回当前值
		if s, ok := current.(string); ok {
			return s
		}
		// 对于其他类型，检查是否为 nil
		if current == nil {
			return ""
		}
		return fmt.Sprintf("%v", current)
	}

	field := fields[idx]

	switch v := current.(type) {
	case map[string]interface{}:
		if val, ok := v[field]; ok {
			return c.extractFromFields(val, fields, idx+1)
		}
		return ""
	case []interface{}:
		// 尝试将字段解析为数组索引
		idxNum, err := strconv.Atoi(field)
		if err != nil {
			return ""
		}
		if idxNum < 0 || idxNum >= len(v) {
			return ""
		}
		return c.extractFromFields(v[idxNum], fields, idx+1)
	case string:
		// 已经是字符串，返回它
		if idx == len(fields)-1 {
			return v
		}
		return ""
	default:
		// 其他类型返回空字符串
		return ""
	}
}

// Check 检查 API 是否可用
func (c *RemoteClient) Check(ctx context.Context) error {
	// 尝试获取模型列表
	url := c.baseURL + "/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	for key, value := range c.requestCfg.Headers {
		req.Header.Set(key, value)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("无法连接到远程 AI API：%w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API 返回错误状态：%d", resp.StatusCode)
	}

	return nil
}

// GetModel 实现 AIService.GetModel
func (c *RemoteClient) GetModel() string {
	return c.model
}
