package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAIConfigWithoutLocalOverlayInTempDir(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile(filepath.Join("..", "..", "config", "ai_config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "ai_config.yaml")
	if err := os.WriteFile(base, raw, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAIConfig(base); err != nil {
		t.Fatalf("LoadAIConfig without local overlay should succeed: %v", err)
	}
}

func TestLoadAIConfigMergesLocalOverlay(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "ai_config.yaml")
	local := filepath.Join(dir, localAIConfigName)
	if err := os.WriteFile(base, []byte(`
planner:
  mode: remote
  remote:
    base_url: ${AI_BASE_URL}
    api_key: ${AI_API_KEY}
    model: base-model
worker:
  mode: remote
  remote:
    model: worker-model
monitor:
  mode: remote
  remote:
    model: monitor-model
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte(`
planner:
  remote:
    base_url: http://secret-host:9101
    api_key: secret-key
worker:
  mode: local
  local:
    enabled: true
    model: local-worker
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAIConfig(base)
	if err != nil {
		t.Fatalf("LoadAIConfig failed: %v", err)
	}
	if cfg.Planner.Remote.BaseURL != "http://secret-host:9101" {
		t.Fatalf("planner base_url = %q", cfg.Planner.Remote.BaseURL)
	}
	if cfg.Planner.Remote.APIKey != "secret-key" {
		t.Fatalf("planner api_key = %q", cfg.Planner.Remote.APIKey)
	}
	if !cfg.IsWorkerLocal() || cfg.Worker.Local.Model != "local-worker" {
		t.Fatalf("worker local merge failed: mode=%s model=%s", cfg.Worker.Mode, cfg.Worker.Local.Model)
	}
}

func TestWorkerAIFallbackToPlannerEnv(t *testing.T) {
	t.Setenv("WORKER_AI_BASE_URL", "")
	t.Setenv("WORKER_AI_API_KEY", "")
	t.Setenv("AI_BASE_URL", "https://api.deepseek.com")
	t.Setenv("AI_API_KEY", "sk-test-key")

	dir := t.TempDir()
	base := filepath.Join(dir, "ai_config.yaml")
	if err := os.WriteFile(base, []byte(`
planner:
  mode: remote
  remote:
    base_url: ${AI_BASE_URL}
    api_key: ${AI_API_KEY}
    model: planner-model
worker:
  mode: remote
  remote:
    base_url: ${WORKER_AI_BASE_URL}
    api_key: ${WORKER_AI_API_KEY}
    model: worker-model
monitor:
  mode: remote
  remote:
    model: monitor-model
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAIConfig(base)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Worker.Remote.BaseURL != "https://api.deepseek.com" {
		t.Fatalf("worker base_url = %q", cfg.Worker.Remote.BaseURL)
	}
	if cfg.Worker.Remote.APIKey != "sk-test-key" {
		t.Fatalf("worker api_key = %q", cfg.Worker.Remote.APIKey)
	}
}
