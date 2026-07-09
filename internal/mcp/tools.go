package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, input map[string]interface{}) (string, error)
}

type ShellExecTool struct{}

func (t *ShellExecTool) Name() string { return "shell_exec" }
func (t *ShellExecTool) Description() string {
	return `Execute a shell command. Input: {"command": "ls -la", "workdir": "/tmp"}`
}
func (t *ShellExecTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	command, _ := input["command"].(string)
	workdir, _ := input["workdir"].(string)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}
	if workdir == "" {
		workdir = "/tmp"
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = workdir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out) + "\n[ERROR] " + err.Error(), nil
	}
	return string(out), nil
}

type FileReadTool struct{}

func (t *FileReadTool) Name() string { return "file_read" }
func (t *FileReadTool) Description() string {
	return `Read a file. Input: {"path": "/data/file.txt"}`
}
func (t *FileReadTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, err := resolveSafePath(stringVal(input, "path"))
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file failed: %w", err)
	}
	if len(data) > 100*1024 {
		return string(data[:100*1024]) + "\n...[truncated]", nil
	}
	return string(data), nil
}

type FileWriteTool struct{}

func (t *FileWriteTool) Name() string { return "file_write" }
func (t *FileWriteTool) Description() string {
	return `Write content to a file. Input: {"path": "/data/out.txt", "content": "hello"}`
}
func (t *FileWriteTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, err := resolveSafePath(stringVal(input, "path"))
	if err != nil {
		return "", err
	}
	content := stringVal(input, "content")
	if err := ensureParentDir(path); err != nil {
		return "", fmt.Errorf("mkdir failed: %w", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write file failed: %w", err)
	}
	return fmt.Sprintf("Written %d bytes to %s", len(content), path), nil
}

type HTTPRequestTool struct {
	Client *http.Client
}

func (t *HTTPRequestTool) Name() string { return "http_request" }
func (t *HTTPRequestTool) Description() string {
	return `Make an HTTP request. Input: {"method": "GET", "url": "https://example.com", "headers": {"Auth": "xxx"}, "body": "..."}`
}
func (t *HTTPRequestTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	method, _ := input["method"].(string)
	url, _ := input["url"].(string)
	if method == "" {
		method = "GET"
	}
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	var body io.Reader
	if bodyStr, ok := input["body"].(string); ok && bodyStr != "" {
		body = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	if headers, ok := input["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			req.Header.Set(k, fmt.Sprintf("%v", v))
		}
	}
	if req.Header.Get("Content-Type") == "" && body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return "", fmt.Errorf("read response failed: %w", err)
	}

	return fmt.Sprintf("Status: %d\n%s", resp.StatusCode, string(respBody)), nil
}

type ListDirTool struct{}

func (t *ListDirTool) Name() string { return "list_dir" }
func (t *ListDirTool) Description() string {
	return `List directory contents. Input: {"path": "/data"}`
}
func (t *ListDirTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	pathInput := stringVal(input, "path")
	if pathInput == "" {
		pathInput = "/tmp"
	}
	path, err := resolveSafePath(pathInput)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("list dir failed: %w", err)
	}
	var lines []string
	for _, e := range entries {
		info, _ := e.Info()
		if info != nil {
			lines = append(lines, fmt.Sprintf("%s  %s  %d bytes", e.Name(), e.Type(), info.Size()))
		} else {
			lines = append(lines, fmt.Sprintf("%s  %s", e.Name(), e.Type()))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func DefaultTools() []Tool {
	outputDir := os.Getenv("OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "/data/outputs"
	}

	webClient := &http.Client{Timeout: defaultWebClientTimeout}
	tools := []Tool{
		&ShellExecTool{},
		&FileReadTool{},
		&FileWriteTool{},
		&FileAppendTool{},
		&FileDeleteTool{},
		&FileExistsTool{},
		&FileCopyTool{},
		&ListDirTool{},
		&GlobFindTool{},
		&GrepSearchTool{},
		&HTTPRequestTool{Client: &http.Client{Timeout: 30 * time.Second}},
		&HTTPDownloadTool{Client: &http.Client{Timeout: 60 * time.Second}},
		&EnvGetTool{},
		&JSONQueryTool{},
		&TextEncodeTool{},
		&HashTextTool{},
		&SleepWaitTool{},
		&WriteOutputTool{OutputDir: outputDir},
	}
	tools = append(tools, webTools(webClient)...)
	if k8sTool, err := NewK8sGetTool(); err == nil {
		tools = append(tools, k8sTool)
	}
	return tools
}
