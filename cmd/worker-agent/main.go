package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	applog "agent-flow/internal/log"
	"agent-flow/internal/mcp"
)

func main() {
	applog.InitFromEnv()
	log := applog.Component("worker-agent")

	mcpURL := flag.String("mcp-url", getEnv("MCP_URL", "http://localhost:9090"), "MCP server URL")
	taskDesc := flag.String("task", getEnv("TASK_DESC", ""), "Task description")
	taskName := flag.String("task-name", getEnv("TASK_NAME", "unknown"), "Task name")
	outputDir := flag.String("output-dir", getEnv("OUTPUT_DIR", "/data/outputs"), "Output directory")
	maxSteps := flag.Int("max-steps", 10, "Max agent steps")
	flag.Parse()

	if *taskDesc == "" {
		log.Error("TASK_DESC is required")
		os.Exit(1)
	}

	log.Info("starting worker agent",
		"task", *taskName,
		"mcpURL", *mcpURL,
		"taskDesc", *taskDesc,
	)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	agent := mcp.NewAgent(mcp.AgentConfig{
		MCPURL:   *mcpURL,
		MaxSteps: *maxSteps,
	})

	toolDesc, err := agent.FetchToolCatalog(ctx)
	if err != nil {
		log.Warn("failed to fetch tool catalog, using defaults", "error", err)
		toolDesc = mcp.FormatToolCatalog(mcp.DefaultTools())
	}
	log.Debug("available tools", "catalog", toolDesc)

	result, runErr := agent.Run(ctx, *taskDesc, toolDesc)
	if runErr != nil {
		log.Error("agent run failed", "error", runErr)
	}

	output := result.Output
	if output == "" && len(result.Steps) > 0 {
		output = result.Steps[len(result.Steps)-1].Thought
	}

	outputPath := fmt.Sprintf("%s/%s.txt", *outputDir, *taskName)
	if err := os.WriteFile(outputPath, []byte(output), 0644); err != nil {
		log.Error("failed to write output", "path", outputPath, "error", err)
	} else {
		log.Info("output written", "path", outputPath, "bytes", len(output))
	}

	log.Info("agent completed", "steps", len(result.Steps))
	for i, step := range result.Steps {
		thought := step.Thought
		if len(thought) > 100 {
			thought = thought[:100]
		}
		log.Debug("agent step", "index", i, "thought", thought, "tool", step.Tool)
	}

	if runErr != nil {
		os.Exit(1)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}