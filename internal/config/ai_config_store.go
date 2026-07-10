package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const apiKeyUnchangedSentinel = "__UNCHANGED__"

// AIRemoteSettings is the API-facing remote AI configuration.
type AIRemoteSettings struct {
	Enabled        bool    `json:"enabled" yaml:"enabled"`
	BaseURL        string  `json:"base_url" yaml:"base_url"`
	APIKey         string  `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	APIKeySet      bool    `json:"api_key_set" yaml:"-"`
	Model          string  `json:"model" yaml:"model"`
	Temperature    float64 `json:"temperature" yaml:"temperature"`
	MaxTokens      int     `json:"max_tokens" yaml:"max_tokens"`
	TimeoutSeconds int     `json:"timeout_seconds" yaml:"timeout_seconds"`
}

// AILocalSettings is the API-facing local AI configuration.
type AILocalSettings struct {
	Enabled        bool    `json:"enabled" yaml:"enabled"`
	BaseURL        string  `json:"base_url" yaml:"base_url"`
	Model          string  `json:"model" yaml:"model"`
	Temperature    float64 `json:"temperature" yaml:"temperature"`
	TopP           float64 `json:"top_p" yaml:"top_p"`
	MaxTokens      int     `json:"max_tokens" yaml:"max_tokens"`
	TimeoutSeconds int     `json:"timeout_seconds" yaml:"timeout_seconds"`
}

// RoleAISettings is the API-facing per-role AI configuration.
type RoleAISettings struct {
	Mode         string           `json:"mode" yaml:"mode"`
	Description  string           `json:"description,omitempty" yaml:"description,omitempty"`
	SystemPrompt string           `json:"system_prompt,omitempty" yaml:"system_prompt,omitempty"`
	Remote       AIRemoteSettings `json:"remote" yaml:"remote"`
	Local        AILocalSettings  `json:"local" yaml:"local"`
}

// AISettingsView is returned by the settings API (secrets masked).
type AISettingsView struct {
	ConfigPath string         `json:"config_path"`
	LocalPath  string         `json:"local_path"`
	Planner    RoleAISettings `json:"planner"`
	Worker     RoleAISettings `json:"worker"`
	Monitor    RoleAISettings `json:"monitor"`
	Quality    QualityConfig  `json:"quality"`
	Retry      RetryConfig    `json:"retry"`
}

// AISettingsUpdate is accepted by PUT /settings/ai.
type AISettingsUpdate struct {
	Planner RoleAISettings `json:"planner"`
	Worker  RoleAISettings `json:"worker"`
	Monitor RoleAISettings `json:"monitor"`
	Quality QualityConfig  `json:"quality"`
	Retry   RetryConfig    `json:"retry"`
}

// LocalAIConfigPath returns the path to ai_config.local.yaml next to the base config.
func LocalAIConfigPath(basePath string) string {
	return filepath.Join(filepath.Dir(basePath), localAIConfigName)
}

// ToAISettingsView converts a loaded config into an API-safe view.
func ToAISettingsView(cfg *AIConfig, basePath string) AISettingsView {
	if cfg == nil {
		return AISettingsView{}
	}
	return AISettingsView{
		ConfigPath: basePath,
		LocalPath:  LocalAIConfigPath(basePath),
		Planner:    toRoleSettings(cfg.Planner.Mode, cfg.Planner.Description, cfg.Planner.SystemPrompt, cfg.Planner.Remote, cfg.Planner.Local),
		Worker:     toRoleSettings(cfg.Worker.Mode, cfg.Worker.Description, cfg.Worker.SystemPrompt, cfg.Worker.Remote, cfg.Worker.Local),
		Monitor:    toRoleSettings(cfg.Monitor.Mode, cfg.Monitor.Description, cfg.Monitor.SystemPrompt, cfg.Monitor.Remote, cfg.Monitor.Local),
		Quality:    cfg.Quality,
		Retry:      cfg.Retry,
	}
}

func toRoleSettings(mode, description, systemPrompt string, remote RemoteConfig, local LocalConfig) RoleAISettings {
	return RoleAISettings{
		Mode:         mode,
		Description:  description,
		SystemPrompt: systemPrompt,
		Remote: AIRemoteSettings{
			Enabled:        remote.Enabled,
			BaseURL:        remote.BaseURL,
			APIKeySet:      strings.TrimSpace(remote.APIKey) != "",
			Model:          remote.Model,
			Temperature:    remote.Temperature,
			MaxTokens:      remote.MaxTokens,
			TimeoutSeconds: remote.TimeoutSeconds,
		},
		Local: AILocalSettings{
			Enabled:        local.Enabled,
			BaseURL:        local.BaseURL,
			Model:          local.Model,
			Temperature:    local.Temperature,
			TopP:           local.TopP,
			MaxTokens:      local.MaxTokens,
			TimeoutSeconds: local.TimeoutSeconds,
		},
	}
}

// BuildLocalOverlayFromUpdate merges an API update into a local overlay config.
func BuildLocalOverlayFromUpdate(current *AIConfig, update AISettingsUpdate) *AIConfig {
	overlay := &AIConfig{
		Planner: PlannerConfig{
			Mode:         update.Planner.Mode,
			Description:  update.Planner.Description,
			SystemPrompt: update.Planner.SystemPrompt,
			Remote:       roleRemoteToConfig(update.Planner.Remote, currentRoleRemote(current, "planner")),
			Local:        roleLocalToConfig(update.Planner.Local),
		},
		Worker: WorkerConfig{
			Mode:         update.Worker.Mode,
			Description:  update.Worker.Description,
			SystemPrompt: update.Worker.SystemPrompt,
			Remote:       roleRemoteToConfig(update.Worker.Remote, currentRoleRemote(current, "worker")),
			Local:        roleLocalToConfig(update.Worker.Local),
		},
		Monitor: MonitorConfig{
			Mode:         update.Monitor.Mode,
			Description:  update.Monitor.Description,
			SystemPrompt: update.Monitor.SystemPrompt,
			Remote:       roleRemoteToConfig(update.Monitor.Remote, currentRoleRemote(current, "monitor")),
			Local:        roleLocalToConfig(update.Monitor.Local),
		},
		Quality: update.Quality,
		Retry:   update.Retry,
	}
	return overlay
}

func currentRoleRemote(cfg *AIConfig, role string) RemoteConfig {
	if cfg == nil {
		return RemoteConfig{}
	}
	switch role {
	case "planner":
		return cfg.Planner.Remote
	case "worker":
		return cfg.Worker.Remote
	case "monitor":
		return cfg.Monitor.Remote
	default:
		return RemoteConfig{}
	}
}

func roleRemoteToConfig(in AIRemoteSettings, current RemoteConfig) RemoteConfig {
	out := RemoteConfig{
		Enabled:        in.Enabled,
		BaseURL:        in.BaseURL,
		Model:          in.Model,
		Temperature:    in.Temperature,
		MaxTokens:      in.MaxTokens,
		TimeoutSeconds: in.TimeoutSeconds,
	}
	apiKey := strings.TrimSpace(in.APIKey)
	switch {
	case apiKey == "" || apiKey == apiKeyUnchangedSentinel:
		out.APIKey = current.APIKey
	default:
		out.APIKey = apiKey
	}
	return out
}

func roleLocalToConfig(in AILocalSettings) LocalConfig {
	return LocalConfig{
		Enabled:        in.Enabled,
		BaseURL:        in.BaseURL,
		Model:          in.Model,
		Temperature:    in.Temperature,
		TopP:           in.TopP,
		MaxTokens:      in.MaxTokens,
		TimeoutSeconds: in.TimeoutSeconds,
	}
}

// SaveAIConfigLocal writes role settings to ai_config.local.yaml.
func SaveAIConfigLocal(basePath string, overlay *AIConfig) error {
	if overlay == nil {
		return fmt.Errorf("overlay config is nil")
	}
	localPath := LocalAIConfigPath(basePath)
	data, err := yaml.Marshal(overlay)
	if err != nil {
		return fmt.Errorf("序列化本地 AI 配置失败：%w", err)
	}
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败：%w", err)
	}
	if err := os.WriteFile(localPath, data, 0600); err != nil {
		return fmt.Errorf("写入本地 AI 配置失败：%w", err)
	}
	return nil
}