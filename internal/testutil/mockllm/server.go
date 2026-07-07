// Package mockllm provides an OpenAI-compatible HTTP server for CI E2E tests.
package mockllm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
)

// Server is a test HTTP server mimicking /v1/chat/completions.
type Server struct {
	URL    string
	server *httptest.Server
	calls  atomic.Int32
}

// New starts a mock LLM server with keyword-based canned responses.
func New() *Server {
	s := &Server{}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	s.URL = s.server.URL
	return s
}

// Close shuts down the server.
func (s *Server) Close() {
	s.server.Close()
}

// Calls returns how many chat requests were served.
func (s *Server) Calls() int {
	return int(s.calls.Load())
}

type chatRequest struct {
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/chat/completions" {
		http.NotFound(w, r)
		return
	}
	s.calls.Add(1)

	var req chatRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	prompt := strings.Builder{}
	for _, m := range req.Messages {
		prompt.WriteString(m.Content)
		prompt.WriteString("\n")
	}
	text := prompt.String()
	content := responseFor(text)

	resp := map[string]interface{}{
		"choices": []map[string]interface{}{
			{"message": map[string]string{"role": "assistant", "content": content}},
		},
		"usage": map[string]int{
			"prompt_tokens":     120,
			"completion_tokens": 80,
			"total_tokens":      200,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func responseFor(prompt string) string {
	lower := strings.ToLower(prompt)
	switch {
	case strings.Contains(prompt, "质量检查") || strings.Contains(prompt, "监工") || strings.Contains(lower, "score"):
		return `{"score": 85, "passed": true, "feedback": "mock pass", "issues": [], "dimensions": {"completeness": 30, "accuracy": 28, "quality": 27}}`
	case strings.Contains(prompt, "大纲") || strings.Contains(lower, "outline"):
		return `{"title":"荒岛求生记","synopsis":"一群幸存者在荒岛上求生的故事。","chapters":[{"num":1,"title":"风暴","summary":"海难登陆"},{"num":2,"title":"营地","summary":"搭建庇护所"}]}`
	default:
		return strings.Repeat("海风呼啸，林涛在礁石间寻找淡水与庇护。对话与动作推进剧情。", 30)
	}
}