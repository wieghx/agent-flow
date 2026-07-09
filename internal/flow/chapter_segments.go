package flow

import (
	"context"
	"fmt"
	"strings"

	"agent-flow/internal/ai"
	"agent-flow/internal/retry"
	wfengine "agent-flow/internal/workflow"
)

// ChapterWriter performs worker AI calls for segmented chapter generation.
type ChapterWriter interface {
	WorkerChat(ctx context.Context, systemPrompt, userMessage string) (ai.ChatResult, error)
}

// ShouldUseSegmentedChapter reports whether instruction requests segmented writing.
func ShouldUseSegmentedChapter(instruction, monitorTaskType string) bool {
	_, _, ok := wfengine.ParseSegmentConfig(instruction)
	return ok
}

// GenerateSegmentedChapter writes a chapter in multiple short AI passes and stitches the result.
func GenerateSegmentedChapter(ctx context.Context, writer ChapterWriter, instruction, monitorFeedback string) (string, ai.TokenUsage, error) {
	if writer == nil {
		return "", ai.TokenUsage{}, fmt.Errorf("AI 服务未初始化")
	}
	var totalUsage ai.TokenUsage
	segments, segmentWords, ok := wfengine.ParseSegmentConfig(instruction)
	if !ok {
		target := ParseTargetWordsFromInstruction(instruction)
		segments = wfengine.SegmentCount(nil, target)
		segmentWords = wfengine.SegmentWordsPerPart(nil, target)
	}
	if segments <= 0 || segmentWords <= 0 {
		return "", totalUsage, fmt.Errorf("invalid segment config: segments=%d segmentWords=%d", segments, segmentWords)
	}

	systemPrompt := buildWorkerSystemPrompt(instruction)
	var parts []string
	var priorTail string
	var openingSample string
	minSeg := wfengine.MinSegmentRunes(segmentWords)

	for i := 1; i <= segments; i++ {
		prose, segUsage, err := writeSegmentWithRetry(ctx, writer, systemPrompt, instruction, i, segments, segmentWords, priorTail, openingSample, monitorFeedback, minSeg)
		if err != nil {
			return "", totalUsage, err
		}
		totalUsage.Add(segUsage)
		parts = append(parts, prose)
		if i == 1 {
			openingSample = wfengine.ChapterOpening(prose, wfengine.SegmentOpeningSampleRunes)
		}
		priorTail = wfengine.ChapterEnding(prose, wfengine.SegmentPriorTailRunes)
	}

	chapter := wfengine.StitchChapterSegments(parts)
	if chapter == "" {
		return "", totalUsage, fmt.Errorf("stitched chapter is empty")
	}
	if ok, names := wfengine.ValidateCharacterPresence(instruction, chapter); !ok {
		return "", totalUsage, fmt.Errorf("stitched chapter missing listed characters (%s)", strings.Join(names, ", "))
	}
	return chapter, totalUsage, nil
}

func writeSegmentWithRetry(
	ctx context.Context,
	writer ChapterWriter,
	systemPrompt, instruction string,
	segmentIndex, totalSegments, segmentWords int,
	priorTail, openingSample, monitorFeedback string,
	minSeg int,
) (string, ai.TokenUsage, error) {
	var totalUsage ai.TokenUsage
	extraFeedback := ""
	if monitorFeedback != "" && segmentIndex == 1 {
		extraFeedback = monitorFeedback
	}

	for attempt := 1; attempt <= retry.DefaultSegmentTries; attempt++ {
		userMessage := wfengine.BuildSegmentInstruction(instruction, segmentIndex, totalSegments, segmentWords, priorTail, openingSample)
		if extraFeedback != "" {
			userMessage = fmt.Sprintf("%s\n\n上次执行的反馈（请根据反馈改进）: %s", userMessage, extraFeedback)
		}

		result, err := writer.WorkerChat(ctx, systemPrompt, userMessage)
		if err != nil {
			kind := retry.Classify(err)
			if attempt < retry.DefaultSegmentTries && retry.IsRetryable(kind) {
				if err := retry.Sleep(ctx, retry.Backoff(attempt, 2, 15)); err != nil {
					return "", totalUsage, err
				}
				continue
			}
			return "", totalUsage, fmt.Errorf("segment %d/%d AI call failed: %w", segmentIndex, totalSegments, err)
		}
		totalUsage.Add(result.Usage)

		prose := ExtractChineseProse(NormalizeWorkerOutput(userMessage, result.Content))
		runes := len([]rune(prose))
		if runes < minSeg {
			extraFeedback = retry.SegmentFeedback(segmentIndex, totalSegments, runes, minSeg)
			if attempt < retry.DefaultSegmentTries {
				if err := retry.Sleep(ctx, retry.Backoff(attempt, 2, 15)); err != nil {
					return "", totalUsage, err
				}
				continue
			}
			return "", totalUsage, fmt.Errorf("segment %d/%d too short (%d runes, need >= %d)", segmentIndex, totalSegments, runes, minSeg)
		}
		if LooksTruncated(prose) {
			extraFeedback = fmt.Sprintf("第 %d/%d 段结尾不完整，请以完整句子收束本段", segmentIndex, totalSegments)
			if attempt < retry.DefaultSegmentTries {
				if err := retry.Sleep(ctx, retry.Backoff(attempt, 2, 15)); err != nil {
					return "", totalUsage, err
				}
				continue
			}
			return "", totalUsage, fmt.Errorf("segment %d/%d appears truncated", segmentIndex, totalSegments)
		}
		return prose, totalUsage, nil
	}
	return "", totalUsage, fmt.Errorf("segment %d/%d failed after %d attempts", segmentIndex, totalSegments, retry.DefaultSegmentTries)
}
