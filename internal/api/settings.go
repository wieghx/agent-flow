package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"agent-flow/internal/config"
)

func (a *API) handleAISettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.handleGetAISettings(w, r)
	case http.MethodPut:
		a.handlePutAISettings(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *API) handleGetAISettings(w http.ResponseWriter, r *http.Request) {
	if a.aiService == nil {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, Response{Success: false, Error: "AI service unavailable"})
		return
	}

	cfg, err := config.LoadAIConfig(a.aiConfigPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, Response{
		Success: true,
		Data:    config.ToAISettingsView(cfg, a.aiConfigPath),
	})
}

func (a *API) handlePutAISettings(w http.ResponseWriter, r *http.Request) {
	if a.aiService == nil {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, Response{Success: false, Error: "AI service unavailable"})
		return
	}

	var update config.AISettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	if err := validateAISettingsUpdate(update); err != nil {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	current, err := config.LoadAIConfig(a.aiConfigPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	overlay := config.BuildLocalOverlayFromUpdate(current, update)
	if err := config.SaveAIConfigLocal(a.aiConfigPath, overlay); err != nil {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}

	reloaded, err := config.LoadAIConfig(a.aiConfigPath)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, Response{Success: false, Error: err.Error()})
		return
	}
	if err := a.aiService.Reload(reloaded); err != nil {
		w.Header().Set("Content-Type", "application/json")
		writeJSON(w, Response{Success: false, Error: "配置已保存，但热重载失败：" + err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, Response{
		Success: true,
		Message: "AI 配置已保存并生效",
		Data:    config.ToAISettingsView(reloaded, a.aiConfigPath),
	})
}

func validateAISettingsUpdate(update config.AISettingsUpdate) error {
	for _, role := range []struct {
		name string
		mode string
	}{
		{"planner", update.Planner.Mode},
		{"worker", update.Worker.Mode},
		{"monitor", update.Monitor.Mode},
	} {
		mode := strings.TrimSpace(strings.ToLower(role.mode))
		if mode != "" && mode != "remote" && mode != "local" {
			return errBadRequest(role.name + " mode must be remote or local")
		}
	}
	return nil
}

type badRequestError string

func (e badRequestError) Error() string { return string(e) }

func errBadRequest(msg string) error { return badRequestError(msg) }