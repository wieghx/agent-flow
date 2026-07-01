package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var allowedPathPrefixes = []string{
	"/data/",
	"/data",
	"/tmp/",
	"/tmp",
	"/workspace/",
	"/workspace",
}

// resolveSafePath resolves and validates a path against sandbox allowlist.
func resolveSafePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		clean = filepath.Clean(filepath.Join("/tmp", clean))
	}

	if strings.Contains(clean, "..") {
		return "", fmt.Errorf("path traversal is not allowed: %s", path)
	}

	for _, prefix := range allowedPathPrefixes {
		if clean == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(clean, prefix) {
			return clean, nil
		}
	}

	return "", fmt.Errorf("path %s is outside allowed directories (/data, /tmp, /workspace)", clean)
}

func ensureParentDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
}
