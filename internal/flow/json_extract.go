package flow

import (
	"encoding/json"
	"strings"
)

// ExtractJSONObject finds the largest valid JSON object containing all requiredKeys.
func ExtractJSONObject(text string, requiredKeys ...string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	best := ""
	for i := 0; i < len(text); i++ {
		if text[i] != '{' {
			continue
		}
		depth := 0
	jLoop:
		for j := i; j < len(text); j++ {
			switch text[j] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					candidate := strings.TrimSpace(text[i : j+1])
					if jsonObjectMatchesKeys(candidate, requiredKeys) {
						var raw json.RawMessage
						if json.Unmarshal([]byte(candidate), &raw) == nil && len(candidate) > len(best) {
							best = candidate
						}
					}
					break jLoop
				}
			}
		}
	}
	return best
}

func jsonObjectMatchesKeys(candidate string, requiredKeys []string) bool {
	for _, k := range requiredKeys {
		if k == "" {
			continue
		}
		if !strings.Contains(candidate, `"`+k+`"`) {
			return false
		}
	}
	return true
}

// NormalizeWorkerOutput strips reasoning noise for outline JSON tasks.
func NormalizeWorkerOutput(instruction, output string) string {
	lower := strings.ToLower(instruction)
	if !strings.Contains(instruction, "大纲") && !strings.Contains(lower, "outline") && !strings.Contains(instruction, `"chapters"`) {
		return output
	}
	if extracted := ExtractJSONObject(output, "title", "chapters"); extracted != "" {
		return extracted
	}
	if extracted := ExtractJSONObject(output, "chapters"); extracted != "" {
		return extracted
	}
	return output
}
