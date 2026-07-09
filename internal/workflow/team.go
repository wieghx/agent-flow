package workflow

import "strings"

// TeamModeEnabled reports whether the novel team pipeline (bible + polish + multi-gate QC) is on.
func TeamModeEnabled(params map[string]string) bool {
	if params == nil {
		return true
	}
	raw := stringsTrimLower(params["teamMode"])
	if raw == "false" || raw == "0" || raw == "off" || raw == "legacy" {
		return false
	}
	return true
}

// DefaultNovelTemplate returns the workflow template for new novels.
func DefaultNovelTemplate(params map[string]string, prompt string) string {
	if HistoricalResearchEnabled(params, prompt) {
		return "novel-team-historical"
	}
	if TeamModeEnabled(params) {
		return "novel-team-chapters"
	}
	return "novel-outline-chapters"
}

func stringsTrimLower(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
