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