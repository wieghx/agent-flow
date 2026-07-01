package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	applog "agent-flow/internal/log"
)

type MCPServer struct {
	tools    map[string]Tool
	aiConfig *AIConfig
	mux      *http.ServeMux
}

type AIConfig struct {
	BaseURL     string
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
}

type ToolCallRequest struct {
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

type ToolCallResponse struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}

type ChatRequest struct {
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatResponse struct {
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func NewMCPServer(aiConfig *AIConfig) *MCPServer {
	s := &MCPServer{
		tools:    make(map[string]Tool),
		aiConfig: aiConfig,
		mux:      http.NewServeMux(),
	}
	for _, tool := range DefaultTools() {
		s.tools[tool.Name()] = tool
	}
	s.setupRoutes()
	return s
}

func (s *MCPServer) RegisterTool(tool Tool) {
	s.tools[tool.Name()] = tool
}

func (s *MCPServer) setupRoutes() {
	s.mux.HandleFunc("/tools/list", s.handleToolList)
	s.mux.HandleFunc("/tools/call", s.handleToolCall)
	s.mux.HandleFunc("/ai/chat", s.handleAIChat)
	s.mux.HandleFunc("/health", s.handleHealth)
}

func (s *MCPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}
	s.mux.ServeHTTP(w, r)
}

func (s *MCPServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *MCPServer) handleToolList(w http.ResponseWriter, r *http.Request) {
	infos := make([]ToolInfo, 0, len(s.tools))
	for _, t := range s.tools {
		infos = append(infos, ToolInfo{Name: t.Name(), Description: t.Description()})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"tools": infos})
}

func (s *MCPServer) handleToolCall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ToolCallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, ToolCallResponse{Error: err.Error()})
		return
	}
	tool, ok := s.tools[req.Name]
	if !ok {
		writeJSON(w, ToolCallResponse{Error: fmt.Sprintf("tool not found: %s", req.Name)})
		return
	}
	output, err := tool.Execute(r.Context(), req.Input)
	if err != nil {
		writeJSON(w, ToolCallResponse{Success: false, Output: output, Error: err.Error()})
		return
	}
	writeJSON(w, ToolCallResponse{Success: true, Output: output})
}

func (s *MCPServer) handleAIChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.aiConfig == nil || s.aiConfig.BaseURL == "" {
		writeJSON(w, ChatResponse{Error: "AI not configured"})
		return
	}
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, ChatResponse{Error: err.Error()})
		return
	}

	maxTokens := s.aiConfig.MaxTokens
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}
	temp := s.aiConfig.Temperature
	if req.Temperature > 0 {
		temp = req.Temperature
	}

	body := map[string]interface{}{
		"model":       s.aiConfig.Model,
		"messages":    req.Messages,
		"temperature": temp,
		"max_tokens":  maxTokens,
	}
	bodyBytes, _ := json.Marshal(body)

	url := strings.TrimRight(s.aiConfig.BaseURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(r.Context(), "POST", url, strings.NewReader(string(bodyBytes)))
	if err != nil {
		writeJSON(w, ChatResponse{Error: err.Error()})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if s.aiConfig.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.aiConfig.APIKey)
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		writeJSON(w, ChatResponse{Error: fmt.Sprintf("AI request failed: %v", err)})
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		writeJSON(w, ChatResponse{Error: fmt.Sprintf("read AI response failed: %v", err)})
		return
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		writeJSON(w, ChatResponse{Error: fmt.Sprintf("parse AI response failed: %v, body: %s", err, string(respBody[:min(len(respBody), 500)]))})
		return
	}
	if result.Error != nil {
		writeJSON(w, ChatResponse{Error: result.Error.Message})
		return
	}
	if len(result.Choices) == 0 {
		writeJSON(w, ChatResponse{Error: "no choices in AI response"})
		return
	}
	writeJSON(w, ChatResponse{Content: result.Choices[0].Message.Content})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *MCPServer) Start(addr string) error {
	applog.Component("mcp").Info("server starting",
		"addr", addr,
		"endpoints", []string{
			"GET /tools/list",
			"POST /tools/call",
			"POST /ai/chat",
			"GET /health",
		},
	)
	return http.ListenAndServe(addr, s)
}
