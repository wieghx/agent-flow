package main

import (
	"flag"
	"fmt"
	"os"

	applog "agent-flow/internal/log"
	"agent-flow/internal/mcp"
)

func main() {
	applog.InitFromEnv()
	log := applog.Component("mcp-server")

	port := flag.Int("port", 9090, "MCP server port")
	aiBaseURL := flag.String("ai-base-url", getEnv("AI_BASE_URL", ""), "AI API base URL")
	aiAPIKey := flag.String("ai-api-key", getEnv("AI_API_KEY", ""), "AI API key")
	aiModel := flag.String("ai-model", getEnv("AI_MODEL", "Qwen3.5-35B-A3B"), "AI model name")
	aiMaxTokens := flag.Int("ai-max-tokens", 4096, "AI max tokens")
	flag.Parse()

	aiConfig := &mcp.AIConfig{
		BaseURL:     *aiBaseURL,
		APIKey:      *aiAPIKey,
		Model:       *aiModel,
		MaxTokens:   *aiMaxTokens,
		Temperature: 0.7,
	}

	server := mcp.NewMCPServer(aiConfig)

	addr := fmt.Sprintf(":%d", *port)
	log.Info("starting MCP sidecar server", "addr", addr, "aiBaseURL", *aiBaseURL, "aiModel", *aiModel)

	if err := server.Start(addr); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
