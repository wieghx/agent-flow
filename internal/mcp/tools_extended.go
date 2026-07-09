package mcp

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type FileAppendTool struct{}

func (t *FileAppendTool) Name() string { return "file_append" }
func (t *FileAppendTool) Description() string {
	return `Append content to a file (creates if missing). Input: {"path": "/data/log.txt", "content": "line\\n"}`
}
func (t *FileAppendTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, err := resolveSafePath(stringVal(input, "path"))
	if err != nil {
		return "", err
	}
	content := stringVal(input, "content")
	if err := ensureParentDir(path); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(content); err != nil {
		return "", err
	}
	return fmt.Sprintf("Appended %d bytes to %s", len(content), path), nil
}

type FileDeleteTool struct{}

func (t *FileDeleteTool) Name() string { return "file_delete" }
func (t *FileDeleteTool) Description() string {
	return `Delete a file or empty directory. Input: {"path": "/tmp/old.txt"}`
}
func (t *FileDeleteTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, err := resolveSafePath(stringVal(input, "path"))
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return fmt.Sprintf("Deleted %s", path), nil
}

type FileExistsTool struct{}

func (t *FileExistsTool) Name() string { return "file_exists" }
func (t *FileExistsTool) Description() string {
	return `Check whether a path exists. Input: {"path": "/data/out.txt"}`
}
func (t *FileExistsTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	path, err := resolveSafePath(stringVal(input, "path"))
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Sprintf("exists=false path=%s", path), nil
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("exists=true path=%s type=%s size=%d modTime=%s",
		path, fileType(info), info.Size(), info.ModTime().Format(time.RFC3339)), nil
}

type FileCopyTool struct{}

func (t *FileCopyTool) Name() string { return "file_copy" }
func (t *FileCopyTool) Description() string {
	return `Copy a file. Input: {"src": "/data/a.txt", "dst": "/data/b.txt"}`
}
func (t *FileCopyTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	src, err := resolveSafePath(stringVal(input, "src"))
	if err != nil {
		return "", err
	}
	dst, err := resolveSafePath(stringVal(input, "dst"))
	if err != nil {
		return "", err
	}
	if err := ensureParentDir(dst); err != nil {
		return "", err
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer func() { _ = out.Close() }()
	n, err := io.Copy(out, in)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Copied %d bytes from %s to %s", n, src, dst), nil
}

type GlobFindTool struct{}

func (t *GlobFindTool) Name() string { return "glob_find" }
func (t *GlobFindTool) Description() string {
	return `Find files by glob pattern. Input: {"pattern": "/data/**/*.txt", "limit": 50}`
}
func (t *GlobFindTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	pattern := stringVal(input, "pattern")
	if pattern == "" {
		return "", fmt.Errorf("pattern is required")
	}
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join("/tmp", pattern)
	}
	pattern = filepath.Clean(pattern)
	baseDir := pattern
	if idx := strings.Index(pattern, "*"); idx >= 0 {
		baseDir = strings.TrimRight(pattern[:idx], "/")
	}
	if baseDir == "" {
		baseDir = "/tmp"
	}
	if _, err := resolveSafePath(baseDir); err != nil {
		return "", err
	}

	limit := intVal(input, "limit", 50)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	if len(matches) > limit {
		matches = matches[:limit]
	}
	if len(matches) == 0 {
		return "No matches found", nil
	}
	return strings.Join(matches, "\n"), nil
}

type GrepSearchTool struct{}

func (t *GrepSearchTool) Name() string { return "grep_search" }
func (t *GrepSearchTool) Description() string {
	return `Search text in files under a directory. Input: {"path": "/data", "query": "error", "ignore_case": true, "limit": 20}`
}
func (t *GrepSearchTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	root, err := resolveSafePath(stringVal(input, "path"))
	if err != nil {
		return "", err
	}
	query := stringVal(input, "query")
	if query == "" {
		return "", fmt.Errorf("query is required")
	}
	ignoreCase := boolVal(input, "ignore_case")
	limit := intVal(input, "limit", 20)

	if ignoreCase {
		query = strings.ToLower(query)
	}

	var hits []string
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		if len(hits) >= limit {
			return io.EOF
		}
		info, err := d.Info()
		if err != nil || info.Size() > 512*1024 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		text := string(data)
		needle := query
		searchText := text
		if ignoreCase {
			searchText = strings.ToLower(text)
		}
		if !strings.Contains(searchText, needle) {
			return nil
		}
		for i, line := range strings.Split(text, "\n") {
			lineCmp := line
			if ignoreCase {
				lineCmp = strings.ToLower(line)
			}
			if strings.Contains(lineCmp, needle) {
				hits = append(hits, fmt.Sprintf("%s:%d:%s", path, i+1, strings.TrimSpace(line)))
				if len(hits) >= limit {
					return io.EOF
				}
			}
		}
		return nil
	})
	if err != nil && err != io.EOF {
		return "", err
	}
	if len(hits) == 0 {
		return "No matches found", nil
	}
	return strings.Join(hits, "\n"), nil
}

type EnvGetTool struct{}

func (t *EnvGetTool) Name() string { return "env_get" }
func (t *EnvGetTool) Description() string {
	return `Read environment variables. Input: {"name": "TASK_NAME"} or {"names": ["A","B"]} or {} for all`
}
func (t *EnvGetTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	if name := stringVal(input, "name"); name != "" {
		val, ok := os.LookupEnv(name)
		if !ok {
			return fmt.Sprintf("%s=<unset>", name), nil
		}
		return fmt.Sprintf("%s=%s", name, val), nil
	}

	if rawNames, ok := input["names"].([]interface{}); ok {
		lines := make([]string, 0, len(rawNames))
		for _, n := range rawNames {
			name := fmt.Sprintf("%v", n)
			val, ok := os.LookupEnv(name)
			if !ok {
				lines = append(lines, fmt.Sprintf("%s=<unset>", name))
				continue
			}
			lines = append(lines, fmt.Sprintf("%s=%s", name, val))
		}
		return strings.Join(lines, "\n"), nil
	}

	lines := os.Environ()
	if len(lines) > 100 {
		lines = lines[:100]
		lines = append(lines, "...[truncated]")
	}
	return strings.Join(lines, "\n"), nil
}

type JSONQueryTool struct{}

func (t *JSONQueryTool) Name() string { return "json_query" }
func (t *JSONQueryTool) Description() string {
	return `Parse JSON and extract fields. Input: {"json": "{...}", "path": "choices.0.message.content"} or {"path": "/data/resp.json", "field": "status"}`
}
func (t *JSONQueryTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	var raw []byte
	if jsonText := stringVal(input, "json"); jsonText != "" {
		raw = []byte(jsonText)
	} else if filePath := stringVal(input, "path"); filePath != "" {
		safe, err := resolveSafePath(filePath)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(safe)
		if err != nil {
			return "", err
		}
		raw = data
	} else {
		return "", fmt.Errorf("json or path is required")
	}

	var doc interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("invalid json: %w", err)
	}

	field := stringVal(input, "field")
	if field == "" {
		field = stringVal(input, "query")
	}
	if field == "" {
		pretty, _ := json.MarshalIndent(doc, "", "  ")
		if len(pretty) > 100*1024 {
			return string(pretty[:100*1024]) + "\n...[truncated]", nil
		}
		return string(pretty), nil
	}

	value, ok := jsonPathLookup(doc, field)
	if !ok {
		return "", fmt.Errorf("field not found: %s", field)
	}
	switch v := value.(type) {
	case string:
		return v, nil
	default:
		out, _ := json.MarshalIndent(v, "", "  ")
		return string(out), nil
	}
}

type TextEncodeTool struct{}

func (t *TextEncodeTool) Name() string { return "text_encode" }
func (t *TextEncodeTool) Description() string {
	return `Encode/decode text. Input: {"operation": "base64_encode|base64_decode|url_encode|url_decode", "text": "hello"}`
}
func (t *TextEncodeTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	op := strings.ToLower(stringVal(input, "operation"))
	text := stringVal(input, "text")
	switch op {
	case "base64_encode":
		return base64.StdEncoding.EncodeToString([]byte(text)), nil
	case "base64_decode":
		data, err := base64.StdEncoding.DecodeString(text)
		if err != nil {
			return "", err
		}
		return string(data), nil
	case "url_encode":
		return url.QueryEscape(text), nil
	case "url_decode":
		return url.QueryUnescape(text)
	default:
		return "", fmt.Errorf("unsupported operation: %s", op)
	}
}

type HashTextTool struct{}

func (t *HashTextTool) Name() string { return "hash_text" }
func (t *HashTextTool) Description() string {
	return `Hash text content. Input: {"algorithm": "sha256|md5", "text": "hello"}`
}
func (t *HashTextTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	algo := strings.ToLower(stringVal(input, "algorithm"))
	if algo == "" {
		algo = "sha256"
	}
	text := stringVal(input, "text")
	switch algo {
	case "sha256":
		sum := sha256.Sum256([]byte(text))
		return hex.EncodeToString(sum[:]), nil
	case "md5":
		sum := md5.Sum([]byte(text))
		return hex.EncodeToString(sum[:]), nil
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algo)
	}
}

type SleepWaitTool struct{}

func (t *SleepWaitTool) Name() string { return "sleep_wait" }
func (t *SleepWaitTool) Description() string {
	return `Wait for seconds (max 30). Input: {"seconds": 2}`
}
func (t *SleepWaitTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	seconds := intVal(input, "seconds", 1)
	if seconds <= 0 {
		seconds = 1
	}
	if seconds > 30 {
		seconds = 30
	}
	timer := time.NewTimer(time.Duration(seconds) * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-timer.C:
		return fmt.Sprintf("Slept %d seconds", seconds), nil
	}
}

type WriteOutputTool struct {
	OutputDir string
}

func (t *WriteOutputTool) Name() string { return "write_output" }
func (t *WriteOutputTool) Description() string {
	return `Write final task output to PVC. Input: {"filename": "result.txt", "content": "..."}`
}
func (t *WriteOutputTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	filename := stringVal(input, "filename")
	if filename == "" {
		filename = "output.txt"
	}
	filename = filepath.Base(filename)
	content := stringVal(input, "content")

	dir := t.OutputDir
	if dir == "" {
		dir = "/data/outputs"
	}
	if safeDir, err := resolveSafePath(dir); err == nil {
		dir = safeDir
	}
	target := filepath.Join(dir, filename)
	if err := ensureParentDir(target); err != nil {
		return "", err
	}
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		return "", err
	}
	return fmt.Sprintf("Output written to %s (%d bytes)", target, len(content)), nil
}

type HTTPDownloadTool struct {
	Client *http.Client
}

func (t *HTTPDownloadTool) Name() string { return "http_download" }
func (t *HTTPDownloadTool) Description() string {
	return `Download URL content to a file. Input: {"url": "https://example.com/data.json", "path": "/data/file.json"}`
}
func (t *HTTPDownloadTool) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
	rawURL := stringVal(input, "url")
	if rawURL == "" {
		return "", fmt.Errorf("url is required")
	}
	path, err := resolveSafePath(stringVal(input, "path"))
	if err != nil {
		return "", err
	}

	client := t.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
	if err := ensureParentDir(path); err != nil {
		return "", err
	}
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = out.Close() }()
	n, err := io.Copy(out, io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Downloaded %d bytes to %s (status %d)", n, path, resp.StatusCode), nil
}

func jsonPathLookup(doc interface{}, path string) (interface{}, bool) {
	current := doc
	for _, part := range strings.Split(path, ".") {
		if idx, err := strconv.Atoi(part); err == nil {
			arr, ok := current.([]interface{})
			if !ok || idx < 0 || idx >= len(arr) {
				return nil, false
			}
			current = arr[idx]
			continue
		}
		node, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		val, ok := node[part]
		if !ok {
			return nil, false
		}
		current = val
	}
	return current, true
}

func stringVal(input map[string]interface{}, key string) string {
	if v, ok := input[key].(string); ok {
		return v
	}
	if v, ok := input[key]; ok && v != nil {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func intVal(input map[string]interface{}, key string, fallback int) int {
	switch v := input[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return fallback
	}
}

func boolVal(input map[string]interface{}, key string) bool {
	switch v := input[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	default:
		return false
	}
}

func fileType(info os.FileInfo) string {
	if info.IsDir() {
		return "dir"
	}
	return "file"
}
