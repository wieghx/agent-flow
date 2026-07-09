package safepath

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveUnderRoot joins rel under root and rejects path traversal escapes.
func ResolveUnderRoot(root, rel string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("root is required")
	}
	root = filepath.Clean(root)
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("root must be absolute: %s", root)
	}

	rel = strings.TrimSpace(rel)
	if rel == "" || rel == "." {
		return "", fmt.Errorf("path is required")
	}
	if strings.Contains(rel, "\x00") {
		return "", fmt.Errorf("invalid path")
	}

	joined := filepath.Clean(filepath.Join(root, rel))
	rootPrefix := root + string(filepath.Separator)
	if joined != root && !strings.HasPrefix(joined, rootPrefix) {
		return "", fmt.Errorf("path is outside allowed directory: %s", rel)
	}
	return joined, nil
}
