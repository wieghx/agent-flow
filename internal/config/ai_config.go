package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"
)

const localAIConfigName = "ai_config.local.yaml"

// AIConfig AI 配置结构体
type AIConfig struct {
	Global  GlobalConfig  `yaml:"global"`
	Planner PlannerConfig `yaml:"planner"`
	Worker  WorkerConfig  `yaml:"worker"`
	Monitor MonitorConfig `yaml:"monitor"`
	Logging LoggingConfig `yaml:"logging"`
	Retry   RetryConfig   `yaml:"retry"`
	Quality QualityConfig `yaml:"quality"`
}

// GlobalConfig 全局配置
type GlobalConfig struct {
	DefaultModel string `yaml:"default_model"`
	SystemPrompt string `yaml:"system_prompt"`
}

// PlannerConfig 架构配置
type PlannerConfig struct {
	Description string       `yaml:"description"`
	Mode        string       `yaml:"mode"`
	Remote      RemoteConfig `yaml:"remote"`
	Local       LocalConfig  `yaml:"local"`
}

// WorkerConfig 执行者配置
type WorkerConfig struct {
	Description string       `yaml:"description"`
	Mode        string       `yaml:"mode"`
	Remote      RemoteConfig `yaml:"remote"`
	Local       LocalConfig  `yaml:"local"`
}

// MonitorConfig 监工配置
type MonitorConfig struct {
	Description  string       `yaml:"description"`
	Mode         string       `yaml:"mode"`
	SystemPrompt string       `yaml:"system_prompt"`
	Remote       RemoteConfig `yaml:"remote"`
	Local        LocalConfig  `yaml:"local"`
}

// RemoteConfig 远程 API 配置
type RemoteConfig struct {
	Enabled        bool           `yaml:"enabled"`
	BaseURL        string         `yaml:"base_url"`
	APIKey         string         `yaml:"api_key"`
	Model          string         `yaml:"model"`
	Temperature    float64        `yaml:"temperature"`
	MaxTokens      int            `yaml:"max_tokens"`
	TimeoutSeconds int            `yaml:"timeout_seconds"`
	Request        RequestConfig  `yaml:"request"`
	Response       ResponseConfig `yaml:"response"`
}

// RequestConfig 请求配置
type RequestConfig struct {
	Headers      map[string]string `yaml:"headers"`
	BodyTemplate string            `yaml:"body_template"`
}

// ResponseConfig 响应解析配置
type ResponseConfig struct {
	ExtractField string `yaml:"extract_field"`
	ErrorField   string `yaml:"error_field"`
}

// LocalConfig 本地 Ollama 配置
type LocalConfig struct {
	Enabled        bool           `yaml:"enabled"`
	BaseURL        string         `yaml:"base_url"`
	Model          string         `yaml:"model"`
	Temperature    float64        `yaml:"temperature"`
	TopP           float64        `yaml:"top_p"`
	MaxTokens      int            `yaml:"max_tokens"`
	TimeoutSeconds int            `yaml:"timeout_seconds"`
	Endpoints      EndpointConfig `yaml:"endpoints"`
	Chat           ChatConfig     `yaml:"chat"`
}

// EndpointConfig Ollama 端点配置
type EndpointConfig struct {
	Generate string `yaml:"generate"`
	Chat     string `yaml:"chat"`
}

// ChatConfig 聊天配置
type ChatConfig struct {
	SystemPrompt string `yaml:"system_prompt"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level       string `yaml:"level"`
	AIRequests  bool   `yaml:"ai_requests"`
	AIResponses bool   `yaml:"ai_responses"`
	MaxLogs     int    `yaml:"max_logs"`
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries   int `yaml:"max_retries"`
	BaseDelaySec int `yaml:"base_delay_seconds"`
	MaxDelaySec  int `yaml:"max_delay_seconds"`
}

// QualityConfig 质量检查配置
type QualityConfig struct {
	Threshold  int `yaml:"threshold"`
	MaxRetries int `yaml:"max_retries"`
}

// LoadAIConfig loads AI config and overlays ai_config.local.yaml when present.
func LoadAIConfig(path string) (*AIConfig, error) {
	config, err := loadAIConfigFile(path)
	if err != nil {
		return nil, err
	}

	localPath := filepath.Join(filepath.Dir(path), localAIConfigName)
	if overlay, err := loadAIConfigFile(localPath); err == nil {
		mergeAIConfig(config, overlay)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	resolved := resolveEnvironmentVariables(config)
	return &resolved, nil
}

func loadAIConfigFile(path string) (*AIConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败：%w", err)
	}
	var config AIConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败：%w", err)
	}
	return &config, nil
}

func mergeAIConfig(base, overlay *AIConfig) {
	if overlay == nil || base == nil {
		return
	}
	if overlay.Global.DefaultModel != "" {
		base.Global.DefaultModel = overlay.Global.DefaultModel
	}
	if overlay.Global.SystemPrompt != "" {
		base.Global.SystemPrompt = overlay.Global.SystemPrompt
	}
	mergePlannerConfig(&base.Planner, &overlay.Planner)
	mergeWorkerConfig(&base.Worker, &overlay.Worker)
	mergeMonitorConfig(&base.Monitor, &overlay.Monitor)
	if overlay.Logging.Level != "" {
		base.Logging = overlay.Logging
	}
	if overlay.Retry.MaxRetries > 0 {
		base.Retry = overlay.Retry
	}
	if overlay.Quality.Threshold > 0 {
		base.Quality = overlay.Quality
	}
}

func mergePlannerConfig(base, overlay *PlannerConfig) {
	if overlay.Description != "" {
		base.Description = overlay.Description
	}
	if overlay.Mode != "" {
		base.Mode = overlay.Mode
	}
	mergeRemoteConfig(&base.Remote, &overlay.Remote)
	mergeLocalConfig(&base.Local, &overlay.Local)
}

func mergeWorkerConfig(base, overlay *WorkerConfig) {
	if overlay.Description != "" {
		base.Description = overlay.Description
	}
	if overlay.Mode != "" {
		base.Mode = overlay.Mode
	}
	mergeRemoteConfig(&base.Remote, &overlay.Remote)
	mergeLocalConfig(&base.Local, &overlay.Local)
}

func mergeMonitorConfig(base, overlay *MonitorConfig) {
	if overlay.Description != "" {
		base.Description = overlay.Description
	}
	if overlay.Mode != "" {
		base.Mode = overlay.Mode
	}
	if overlay.SystemPrompt != "" {
		base.SystemPrompt = overlay.SystemPrompt
	}
	mergeRemoteConfig(&base.Remote, &overlay.Remote)
	mergeLocalConfig(&base.Local, &overlay.Local)
}

func mergeRemoteConfig(base, overlay *RemoteConfig) {
	if overlay.Enabled {
		base.Enabled = true
	}
	if overlay.BaseURL != "" {
		base.BaseURL = overlay.BaseURL
	}
	if overlay.APIKey != "" {
		base.APIKey = overlay.APIKey
	}
	if overlay.Model != "" {
		base.Model = overlay.Model
	}
	if overlay.Temperature != 0 {
		base.Temperature = overlay.Temperature
	}
	if overlay.MaxTokens > 0 {
		base.MaxTokens = overlay.MaxTokens
	}
	if overlay.TimeoutSeconds > 0 {
		base.TimeoutSeconds = overlay.TimeoutSeconds
	}
	if len(overlay.Request.Headers) > 0 {
		if base.Request.Headers == nil {
			base.Request.Headers = map[string]string{}
		}
		for k, v := range overlay.Request.Headers {
			base.Request.Headers[k] = v
		}
	}
	if overlay.Request.BodyTemplate != "" {
		base.Request.BodyTemplate = overlay.Request.BodyTemplate
	}
	if overlay.Response.ExtractField != "" {
		base.Response.ExtractField = overlay.Response.ExtractField
	}
	if overlay.Response.ErrorField != "" {
		base.Response.ErrorField = overlay.Response.ErrorField
	}
}

func mergeLocalConfig(base, overlay *LocalConfig) {
	if overlay.Enabled {
		base.Enabled = true
	}
	if overlay.BaseURL != "" {
		base.BaseURL = overlay.BaseURL
	}
	if overlay.Model != "" {
		base.Model = overlay.Model
	}
	if overlay.Temperature != 0 {
		base.Temperature = overlay.Temperature
	}
	if overlay.TopP != 0 {
		base.TopP = overlay.TopP
	}
	if overlay.MaxTokens > 0 {
		base.MaxTokens = overlay.MaxTokens
	}
	if overlay.TimeoutSeconds > 0 {
		base.TimeoutSeconds = overlay.TimeoutSeconds
	}
	if overlay.Endpoints.Generate != "" {
		base.Endpoints.Generate = overlay.Endpoints.Generate
	}
	if overlay.Endpoints.Chat != "" {
		base.Endpoints.Chat = overlay.Endpoints.Chat
	}
	if overlay.Chat.SystemPrompt != "" {
		base.Chat.SystemPrompt = overlay.Chat.SystemPrompt
	}
}

// resolveEnvironmentVariables 解析环境变量
func resolveEnvironmentVariables(config *AIConfig) AIConfig {
	// 处理 API Key
	if config.Planner.Remote.APIKey != "" {
		config.Planner.Remote.APIKey = os.ExpandEnv(config.Planner.Remote.APIKey)
	}
	if config.Worker.Remote.APIKey != "" {
		config.Worker.Remote.APIKey = os.ExpandEnv(config.Worker.Remote.APIKey)
	}
	if config.Monitor.Remote.APIKey != "" {
		config.Monitor.Remote.APIKey = os.ExpandEnv(config.Monitor.Remote.APIKey)
	}

	// 处理 BaseURL
	if config.Planner.Remote.BaseURL != "" {
		config.Planner.Remote.BaseURL = os.ExpandEnv(config.Planner.Remote.BaseURL)
	}
	if config.Worker.Remote.BaseURL != "" {
		config.Worker.Remote.BaseURL = os.ExpandEnv(config.Worker.Remote.BaseURL)
	}
	if config.Monitor.Remote.BaseURL != "" {
		config.Monitor.Remote.BaseURL = os.ExpandEnv(config.Monitor.Remote.BaseURL)
	}

	applyWorkerAIFallback(&config.Worker.Remote)

	return *config
}

// applyWorkerAIFallback uses WORKER_AI_* when set, otherwise falls back to AI_*.
func applyWorkerAIFallback(remote *RemoteConfig) {
	if remote == nil {
		return
	}
	if strings.TrimSpace(remote.BaseURL) == "" {
		if v := strings.TrimSpace(os.Getenv("WORKER_AI_BASE_URL")); v != "" {
			remote.BaseURL = v
		} else if v := strings.TrimSpace(os.Getenv("AI_BASE_URL")); v != "" {
			remote.BaseURL = v
		}
	}
	if strings.TrimSpace(remote.APIKey) == "" {
		if v := strings.TrimSpace(os.Getenv("WORKER_AI_API_KEY")); v != "" {
			remote.APIKey = v
		} else if v := strings.TrimSpace(os.Getenv("AI_API_KEY")); v != "" {
			remote.APIKey = v
		}
	}
}

// GetPlannerConfig 获取架构配置
func (c *AIConfig) GetPlannerConfig() *PlannerConfig {
	return &c.Planner
}

// GetWorkerConfig 获取执行者配置
func (c *AIConfig) GetWorkerConfig() *WorkerConfig {
	return &c.Worker
}

// GetMonitorConfig 获取监工配置
func (c *AIConfig) GetMonitorConfig() *MonitorConfig {
	return &c.Monitor
}

// GetRetryConfig returns global retry defaults from ai_config.yaml.
func (c *AIConfig) GetRetryConfig() RetryConfig {
	return c.Retry
}

// GetQualityConfig returns global quality-check defaults from ai_config.yaml.
func (c *AIConfig) GetQualityConfig() QualityConfig {
	return c.Quality
}

// IsPlannerLocal 检查架构是否使用本地 AI
func (c *AIConfig) IsPlannerLocal() bool {
	return c.Planner.Mode == "local" && c.Planner.Local.Enabled
}

// IsWorkerLocal 检查执行者是否使用本地 AI
func (c *AIConfig) IsWorkerLocal() bool {
	return c.Worker.Mode == "local" && c.Worker.Local.Enabled
}

// IsMonitorLocal 检查监工是否使用本地 AI
func (c *AIConfig) IsMonitorLocal() bool {
	return c.Monitor.Mode == "local" && c.Monitor.Local.Enabled
}

// IsAnyLocal 检查是否有任何角色使用本地 AI
func (c *AIConfig) IsAnyLocal() bool {
	return c.IsPlannerLocal() || c.IsWorkerLocal() || c.IsMonitorLocal()
}

// GetDefaultModel 获取默认模型
func (c *AIConfig) GetDefaultModel(role string) string {
	switch role {
	case "planner":
		if c.IsPlannerLocal() {
			return c.Planner.Local.Model
		}
		return c.Planner.Remote.Model
	case "worker":
		if c.IsWorkerLocal() {
			return c.Worker.Local.Model
		}
		return c.Worker.Remote.Model
	case "monitor":
		if c.IsMonitorLocal() {
			return c.Monitor.Local.Model
		}
		return c.Monitor.Remote.Model
	default:
		return c.Global.DefaultModel
	}
}

// GetModel 获取指定角色的模型配置
func (c *AIConfig) GetModel(role string) string {
	switch role {
	case "planner":
		return c.GetPlannerModel()
	case "worker":
		return c.GetWorkerModel()
	case "monitor":
		return c.GetMonitorModel()
	default:
		return c.Global.DefaultModel
	}
}

// GetPlannerModel 获取架构使用的模型
func (c *AIConfig) GetPlannerModel() string {
	if c.IsPlannerLocal() {
		return c.Planner.Local.Model
	}
	return c.Planner.Remote.Model
}

// GetWorkerModel 获取执行者使用的模型
func (c *AIConfig) GetWorkerModel() string {
	if c.IsWorkerLocal() {
		return c.Worker.Local.Model
	}
	return c.Worker.Remote.Model
}

// GetMonitorModel 获取监工使用的模型
func (c *AIConfig) GetMonitorModel() string {
	if c.IsMonitorLocal() {
		return c.Monitor.Local.Model
	}
	return c.Monitor.Remote.Model
}

// ApplyTemplate 应用模板字符串
func (c *AIConfig) ApplyTemplate(templateStr string, data map[string]interface{}) (string, error) {
	tmpl, err := template.New("config").Parse(templateStr)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return "", err
	}

	return sb.String(), nil
}

// ToJSON 将配置转换为 JSON
func (c *AIConfig) ToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// GetTimeout 获取超时时间
func (r *RemoteConfig) GetTimeout() time.Duration {
	return time.Duration(r.TimeoutSeconds) * time.Second
}

// GetLocalTimeout 获取本地超时时间
func (l *LocalConfig) GetTimeout() time.Duration {
	return time.Duration(l.TimeoutSeconds) * time.Second
}
