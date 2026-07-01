package flow

import (
	"fmt"
	"strings"

	wfengine "agent-flow/internal/workflow"
)

// ValidateWorkerOutput ensures worker output meets structural requirements for the task type.
func ValidateWorkerOutput(instruction, output, taskType string) error {
	if taskType == "" {
		taskType = DetectTaskType(instruction)
	}
	output = strings.TrimSpace(output)
	if output == "" {
		return fmt.Errorf("worker output is empty")
	}

	switch taskType {
	case TaskTypeNovelOutline:
		normalized := NormalizeWorkerOutput(instruction, output)
		if _, err := wfengine.ParseOutlineJSON(normalized); err != nil {
			return fmt.Errorf("outline JSON invalid: %w", err)
		}
	case TaskTypeNovelChapter:
		prose := ExtractChineseProse(output)
		runes := len([]rune(prose))
		target := ParseTargetWordsFromInstruction(instruction)
		minRunes := MinChapterRunes(target)
		if runes < minRunes {
			return fmt.Errorf("chapter prose too short (%d runes, need >= %d for target %d)", runes, minRunes, target)
		}
		if LooksTruncated(prose) {
			return fmt.Errorf("chapter prose appears truncated (%d runes, ends mid-sentence)", runes)
		}
	case TaskTypeNovelVolumeOutline:
		start, end, ok := wfengine.ParseVolumeChapterRangeFromInstruction(instruction)
		if !ok {
			return fmt.Errorf("volume chapter range not found in instruction")
		}
		normalized := NormalizeWorkerOutput(instruction, output)
		if err := wfengine.ValidateVolumeOutline(normalized, start, end); err != nil {
			return fmt.Errorf("volume outline JSON invalid: %w", err)
		}
	}
	return nil
}
