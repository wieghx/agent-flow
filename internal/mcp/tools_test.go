package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSafePath(t *testing.T) {
	cases := []struct {
		path    string
		allowed bool
	}{
		{"/data/outputs/a.txt", true},
		{"/tmp/work/file.txt", true},
		{"/etc/passwd", false},
		{"../etc/passwd", false},
	}
	for _, tc := range cases {
		_, err := resolveSafePath(tc.path)
		if tc.allowed && err != nil {
			t.Fatalf("expected allowed path %q, got %v", tc.path, err)
		}
		if !tc.allowed && err == nil {
			t.Fatalf("expected blocked path %q", tc.path)
		}
	}
}

func TestFileWriteReadAppendCopy(t *testing.T) {
	ctx := context.Background()
	// Use /tmp explicitly — os.TempDir() is /var/folders/... on macOS and fails path allowlist.
	dir := "/tmp/mcp-tools-test"
	_ = os.MkdirAll(dir, 0755)
	defer func() { _ = os.RemoveAll(dir) }()

	base := filepath.Join(dir, "a.txt")
	dst := filepath.Join(dir, "b.txt")

	write := &FileWriteTool{}
	if _, err := write.Execute(ctx, map[string]interface{}{"path": base, "content": "hello"}); err != nil {
		t.Fatalf("file_write: %v", err)
	}

	appendTool := &FileAppendTool{}
	if _, err := appendTool.Execute(ctx, map[string]interface{}{"path": base, "content": " world"}); err != nil {
		t.Fatalf("file_append: %v", err)
	}

	read := &FileReadTool{}
	out, err := read.Execute(ctx, map[string]interface{}{"path": base})
	if err != nil || out != "hello world" {
		t.Fatalf("file_read = %q, err = %v", out, err)
	}

	exists := &FileExistsTool{}
	stat, err := exists.Execute(ctx, map[string]interface{}{"path": base})
	if err != nil || !contains(stat, "exists=true") {
		t.Fatalf("file_exists = %q, err = %v", stat, err)
	}

	copyTool := &FileCopyTool{}
	if _, err := copyTool.Execute(ctx, map[string]interface{}{"src": base, "dst": dst}); err != nil {
		t.Fatalf("file_copy: %v", err)
	}
}

func TestJSONQueryAndHash(t *testing.T) {
	ctx := context.Background()

	jq := &JSONQueryTool{}
	out, err := jq.Execute(ctx, map[string]interface{}{
		"json":  `{"status":"ok","count":3}`,
		"field": "status",
	})
	if err != nil || out != "ok" {
		t.Fatalf("json_query = %q, err = %v", out, err)
	}

	hash := &HashTextTool{}
	sum, err := hash.Execute(ctx, map[string]interface{}{"algorithm": "sha256", "text": "hello"})
	if err != nil || len(sum) != 64 {
		t.Fatalf("hash_text = %q, err = %v", sum, err)
	}
}

func TestTextEncode(t *testing.T) {
	ctx := context.Background()
	tool := &TextEncodeTool{}
	encoded, err := tool.Execute(ctx, map[string]interface{}{"operation": "base64_encode", "text": "hi"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := tool.Execute(ctx, map[string]interface{}{"operation": "base64_decode", "text": encoded})
	if err != nil || decoded != "hi" {
		t.Fatalf("decode = %q, err = %v", decoded, err)
	}
}

func TestFormatToolCatalog(t *testing.T) {
	catalog := FormatToolCatalog(DefaultTools())
	if !contains(catalog, "shell_exec") || !contains(catalog, "write_output") {
		t.Fatalf("catalog missing tools: %s", catalog)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
