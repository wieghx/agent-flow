package flow

import (
	"context"
	"fmt"
	"strings"

	"agent-flow/internal/retry"
	wfengine "agent-flow/internal/workflow"
)

// ChapterWriter performs worker AI calls for segmented chapter generation.
type ChapterWriter interface {
	WorkerChat(ctx context.Context, systemPrompt, userMessage string) (string, error)
}

// ShouldUseSegmentedChapter reports whether instruction requests segmented writing.
func ShouldUseSegmentedChapter(instruction, monitorTaskType string) bool {
	_, _, ok := wfengine.ParseSegmentConfig(instruction)
	return ok
}

// GenerateSegmentedChapter writes a chapter in multiple short AI passes and stitches the result.
func GenerateSegmentedChapter(ctx context.Context, writer ChapterWriter, instruction, monitorFeedback string) (string, error) {
	if writer == nil {
		return "", fmt.Errorf("AI 服务未初始化")
	}
	segments, segmentWords, ok := wfengine.ParseSegmentConfig(instruction)
	if !ok {
		target := ParseTargetWordsFromInstruction(instruction)
		segments = wfengine.SegmentCount(nil, target)
		segmentWords = wfengine.SegmentWordsPerPart(nil, target)
	}
	if segments <= 0 || segmentWords <= 0 {
		return "", fmt.Errorf("invalid segment config: segments=%d segmentWords=%d", segments, segmentWords)
	}

	systemPrompt := buildWorkerSystemPrompt(instruction)
	var parts []string
	var priorTail string
	var openingSample string
	minSeg := wfengine.MinSegmentRunes(segmentWords)

	for i := 1; i <= segments; i++ {
		prose, err := writeSegmentWithRetry(ctx, writer, systemPrompt, instruction, i, segments, segmentWords, priorTail, openingSample, monitorFeedback, minSeg)
		if err != nil {
			return "", err
		}
		parts = append(parts, prose)
		if i == 1 {
			openingSample = wfengine.ChapterOpening(prose, wfengine.SegmentOpeningSampleRunes)
		}
		priorTail = wfengine.ChapterEnding(prose, wfengine.SegmentPriorTailRunes)
	}

	chapter := wfengine.StitchChapterSegments(parts)
	if chapter == "" {
		return "", fmt.Errorf("stitched chapter is empty")
	}
	if ok, names := wfengine.ValidateCharacterPresence(instruction, chapter); !ok {
		return "", fmt.Errorf("stitched chapter missing listed characters (%s)", strings.Join(names, ", "))
	}
	return chapter, nil
}

func writeSegmentWithRetry(
	ctx context.Context,
	writer ChapterWriter,
	systemPrompt, instruction string,
	segmentIndex, totalSegments, segmentWords int,
	priorTail, openingSample, monitorFeedback string,
	minSeg int,
) (string, error) {
	extraFeedback := ""
	if monitorFeedback != "" && segmentIndex == 1 {
		extraFeedback = monitorFeedback
	}

	for attempt := 1; attempt <= retry.DefaultSegmentTries; attempt++ {
		userMessage := wfengine.BuildSegmentInstruction(instruction, segmentIndex, totalSegments, segmentWords, priorTail, openingSample)
		if extraFeedback != "" {
			userMessage = fmt.Sprintf("%s\n\n上次执行的反馈（请根据反馈改进）: %s", userMessage, extraFeedback)
		}

		raw, err := writer.WorkerChat(ctx, systemPrompt, userMessage)
		if err != nil {
			kind := retry.Classify(err)
			if attempt < retry.DefaultSegmentTries && retry.IsRetryable(kind) {
				if err := retry.Sleep(ctx, retry.Backoff(attempt, 2, 15)); err != nil {
					return "", err
				}
				continue
			}
			return "", fmt.Errorf("segment %d/%d AI call failed: %w", segmentIndex, totalSegments, err)
		}

		prose := ExtractChineseProse(NormalizeWorkerOutput(userMessage, raw))
		runes := len([]rune(prose))
		if runes < minSeg {
			extraFeedback = retry.SegmentFeedback(segmentIndex, totalSegments, runes, minSeg)
			if attempt < retry.DefaultSegmentTries {
				if err := retry.Sleep(ctx, retry.Backoff(attempt, 2, 15)); err != nil {
					return "", err
				}
				continue
			}
			return "", fmt.Errorf("segment %d/%d too short (%d runes, need >= %d)", segmentIndex, totalSegments, runes, minSeg)
		}
		if LooksTruncated(prose) {
			extraFeedback = fmt.Sprintf("第 %d/%d 段结尾不完整，请以完整句子收束本段", segmentIndex, totalSegments)
			if attempt < retry.DefaultSegmentTries {
				if err := retry.Sleep(ctx, retry.Backoff(attempt, 2, 15)); err != nil {
					return "", err
				}
				continue
			}
			return "", fmt.Errorf("segment %d/%d appears truncated", segmentIndex, totalSegments)
		}
		return prose, nil
	}
	return "", fmt.Errorf("segment %d/%d failed after %d attempts", segmentIndex, totalSegments, retry.DefaultSegmentTries)
}