package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildLocalOverlayPreservesAPIKey(t *testing.T) {
	current := &AIConfig{
		Planner: PlannerConfig{
			Remote: RemoteConfig{APIKey: "secret-key"},
		},
	}
	update := AISettingsUpdate{
		Planner: RoleAISettings{
			Mode: "remote",
			Remote: AIRemoteSettings{
				BaseURL: "https://api.example.com",
				APIKey:  "",
			},
		},
	}
	overlay := BuildLocalOverlayFromUpdate(current, update)
	if overlay.Planner.Remote.APIKey != "secret-key" {
		t.Fatalf("api key = %q, want secret-key", overlay.Planner.Remote.APIKey)
	}
}

func TestSaveAndLoadAIConfigLocal(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "ai_config.yaml")
	if err := os.WriteFile(base, []byte(`
planner:
  mode: remote
  remote:
    model: base-model
worker:
  mode: remote
monitor:
  mode: remote
`), 0644); err != nil {
		t.Fatal(err)
	}

	overlay := &AIConfig{
		Planner: PlannerConfig{
			Mode: "remote",
			Remote: RemoteConfig{
				BaseURL: "http://localhost:9101",
				APIKey:  "local-key",
				Model:   "custom-model",
			},
		},
	}
	if err := SaveAIConfigLocal(base, overlay); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAIConfig(base)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Planner.Remote.BaseURL != "http://localhost:9101" {
		t.Fatalf("base_url = %q", cfg.Planner.Remote.BaseURL)
	}
	if cfg.Planner.Remote.APIKey != "local-key" {
		t.Fatalf("api_key = %q", cfg.Planner.Remote.APIKey)
	}
	if cfg.Planner.Remote.Model != "custom-model" {
		t.Fatalf("model = %q", cfg.Planner.Remote.Model)
	}
}

func TestToAISettingsViewMasksAPIKey(t *testing.T) {
	view := ToAISettingsView(&AIConfig{
		Planner: PlannerConfig{
			Mode: "remote",
			Remote: RemoteConfig{
				APIKey: "sk-test",
				Model:  "m1",
			},
		},
	}, "/tmp/ai_config.yaml")
	if !view.Planner.Remote.APIKeySet {
		t.Fatal("expected api_key_set=true")
	}
	if view.Planner.Remote.APIKey != "" {
		t.Fatalf("api_key should be empty in view, got %q", view.Planner.Remote.APIKey)
	}
}