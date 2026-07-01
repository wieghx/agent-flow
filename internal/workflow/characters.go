package workflow

import "strings"

// ExtractListedCharacterNames parses bullet-listed character names from prompt context.
func ExtractListedCharacterNames(section string) []string {
	lines := strings.Split(section, "\n")
	var names []string
	inChars := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "设定人物") || strings.Contains(trimmed, "主要人物") {
			inChars = true
			continue
		}
		if inChars {
			if trimmed == "" ||
				strings.HasPrefix(trimmed, "本章大纲") ||
				strings.HasPrefix(trimmed, "当前章节") ||
				strings.HasPrefix(trimmed, "【") {
				break
			}
			if strings.HasPrefix(trimmed, "- ") {
				name := strings.TrimPrefix(trimmed, "- ")
				if paren := strings.Index(name, "（"); paren > 0 {
					name = name[:paren]
				}
				name = strings.TrimSpace(name)
				if name != "" {
					names = append(names, name)
				}
			}
		}
	}
	return names
}

// ValidateCharacterPresence reports whether output mentions at least one listed character.
func ValidateCharacterPresence(instruction, output string) (ok bool, names []string) {
	names = ProtagonistNamesFromInstruction(instruction)
	if len(names) == 0 {
		names = ExtractListedCharacterNames(instruction)
	}
	if len(names) == 0 {
		return true, nil
	}
	for _, name := range names {
		if strings.Contains(output, name) {
			return true, names
		}
	}
	return false, names
}