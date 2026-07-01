package mcp

import (
	"fmt"
	"sort"
	"strings"
)

// FormatToolCatalog returns numbered tool descriptions for the ReAct agent prompt.
func FormatToolCatalog(tools []Tool) string {
	names := make([]string, 0, len(tools))
	byName := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name())
		byName[tool.Name()] = tool
	}
	sort.Strings(names)

	lines := make([]string, 0, len(names))
	for i, name := range names {
		lines = append(lines, fmt.Sprintf("%d. %s - %s", i+1, name, byName[name].Description()))
	}
	return strings.Join(lines, "\n")
}
