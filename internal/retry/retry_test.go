package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClassifyMessage(t *testing.T) {
	cases := map[string]FailureKind{
		"chapter prose too short (635 runes)": FailureTooShort,
		"chapter prose appears truncated":   FailureTruncated,
		"worker output is empty":              FailureEmpty,
		"outline JSON invalid":              FailureInvalid,
		"connection reset by peer":          FailureTransient,
	}
	for msg, want := range cases {
		if got := ClassifyMessage(msg); got != want {
			t.Fatalf("ClassifyMessage(%q) = %q, want %q", msg, got, want)
		}
	}
}

func TestBackoffIncreasesWithAttempt(t *testing.T) {
	d1 := Backoff(1, 5, 60)
	d2 := Backoff(2, 5, 60)
	d3 := Backoff(3, 5, 60)
	if d2 <= d1 {
		t.Fatalf("expected attempt 2 delay > attempt 1, got %v vs %v", d2, d1)
	}
	if d3 <= d2 {
		t.Fatalf("expected attempt 3 delay > attempt 2, got %v vs %v", d3, d2)
	}
	if d3 > 60*time.Second+12*time.Second {
		t.Fatalf("backoff exceeded max: %v", d3)
	}
}

func TestResolvePolicyChapterGetsMoreAttempts(t *testing.T) {
	p := ResolvePolicy(3, 3, Policy{MaxAttempts: 3, BaseDelaySec: 5, MaxDelaySec: 60}, FailureTooShort, true)
	if p.MaxAttempts < 6 {
		t.Fatalf("expected >=6 chapter attempts for too_short, got %d", p.MaxAttempts)
	}
}

func TestSleepRespectsContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := Sleep(ctx, time.Second)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}