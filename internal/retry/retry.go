// Package retry provides shared retry classification, backoff, and feedback helpers.
package retry

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

const (
	DefaultBaseDelaySec = 5
	DefaultMaxDelaySec  = 60
	DefaultSegmentTries = 3
)

// FailureKind classifies why an attempt failed.
type FailureKind string

const (
	FailureEmpty        FailureKind = "empty"
	FailureTooShort     FailureKind = "too_short"
	FailureTruncated    FailureKind = "truncated"
	FailureInvalid      FailureKind = "invalid"
	FailureQuality      FailureKind = "quality"
	FailureTransient    FailureKind = "transient"
	FailureUnknown      FailureKind = "unknown"
	FailureNonRetryable FailureKind = "non_retryable"
)

// Policy configures retry attempts and delays.
type Policy struct {
	MaxAttempts  int
	BaseDelaySec int
	MaxDelaySec  int
}

// Classify maps a validation or execution error to a failure kind.
func Classify(err error) FailureKind {
	if err == nil {
		return FailureUnknown
	}
	return ClassifyMessage(err.Error())
}

// ClassifyMessage maps free-form failure text to a failure kind.
func ClassifyMessage(msg string) FailureKind {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "empty"):
		return FailureEmpty
	case strings.Contains(lower, "too short"), strings.Contains(lower, "prose too short"):
		return FailureTooShort
	case strings.Contains(lower, "truncated"), strings.Contains(lower, "mid-sentence"):
		return FailureTruncated
	case strings.Contains(lower, "invalid"), strings.Contains(lower, "json"):
		return FailureInvalid
	case strings.Contains(lower, "质量"), strings.Contains(lower, "quality"), strings.Contains(lower, "score="):
		return FailureQuality
	case strings.Contains(lower, "timeout"), strings.Contains(lower, "temporar"), strings.Contains(lower, "connection"),
		strings.Contains(lower, "429"), strings.Contains(lower, "503"), strings.Contains(lower, "rate limit"):
		return FailureTransient
	default:
		return FailureUnknown
	}
}

// IsRetryable reports whether another attempt may help.
func IsRetryable(kind FailureKind) bool {
	switch kind {
	case FailureNonRetryable:
		return false
	default:
		return true
	}
}

// Backoff returns exponential delay with jitter for the given 1-based attempt.
func Backoff(attempt int, baseSec, maxSec int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	if baseSec <= 0 {
		baseSec = DefaultBaseDelaySec
	}
	if maxSec <= 0 {
		maxSec = DefaultMaxDelaySec
	}
	delay := time.Duration(baseSec) * time.Second
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay > time.Duration(maxSec)*time.Second {
			delay = time.Duration(maxSec) * time.Second
			break
		}
	}
	jitter := time.Duration(rand.Int63n(int64(delay / 5)))
	return delay + jitter
}

// OutputFeedback builds worker guidance for the next attempt.
func OutputFeedback(kind FailureKind, err error, attempt int) string {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	switch kind {
	case FailureTooShort:
		return fmt.Sprintf("上次产出字数不足（%s）。请显著加长正文，补充场景、对话与细节，直接输出完整章节正文。", msg)
	case FailureTruncated:
		return fmt.Sprintf("上次产出在句中被截断（%s）。请写完整收束的段落，以句号、问号或感叹号结尾。", msg)
	case FailureEmpty:
		return "上次未产出有效正文。请直接输出符合要求的中文正文，不要输出思考过程。"
	case FailureInvalid:
		return fmt.Sprintf("上次产出格式不合格（%s）。请严格按要求输出可直接解析的最终结果。", msg)
	default:
		if msg != "" {
			return fmt.Sprintf("产出未通过校验（%s）。请根据要求修正后重试（第 %d 次改进）。", msg, attempt+1)
		}
		return fmt.Sprintf("产出未通过校验，请修正后重试（第 %d 次改进）。", attempt+1)
	}
}

// SegmentFeedback builds guidance when a single segment is too short.
func SegmentFeedback(segmentIndex, total int, runes, minRunes int) string {
	return fmt.Sprintf(
		"本段（第 %d/%d 段）仅 %d 字，需要至少 %d 字。请加长本段：补充动作、环境、对话与心理描写，只输出本段正文。",
		segmentIndex, total, runes, minRunes,
	)
}

// ResolvePolicy merges task, quality, and global defaults.
func ResolvePolicy(taskMax int32, qualityMax int, global Policy, kind FailureKind, isChapter bool) Policy {
	p := global
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.BaseDelaySec <= 0 {
		p.BaseDelaySec = DefaultBaseDelaySec
	}
	if p.MaxDelaySec <= 0 {
		p.MaxDelaySec = DefaultMaxDelaySec
	}

	max := int(taskMax)
	if max <= 0 && qualityMax > 0 {
		max = qualityMax
	}
	if max <= 0 {
		max = p.MaxAttempts
	}
	if isChapter && max < 5 {
		max = 5
	}
	switch kind {
	case FailureTransient:
		if max < 4 {
			max = 4
		}
	case FailureTooShort, FailureTruncated:
		if isChapter && max < 6 {
			max = 6
		}
	}
	p.MaxAttempts = max
	return p
}

// Sleep waits for backoff or until ctx is cancelled.
func Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
