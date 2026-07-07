package ai

import "testing"

func TestEstimateCostUSD(t *testing.T) {
	got := EstimateCostUSD("deepseek-chat", 1_000_000, 1_000_000)
	want := 0.27 + 1.10
	if got < want-0.001 || got > want+0.001 {
		t.Fatalf("cost = %f, want ~%f", got, want)
	}
}

func TestEstimateCostUSDUnknownModelFallsBack(t *testing.T) {
	got := EstimateCostUSD("unknown-model", 0, 0)
	if got != 0 {
		t.Fatalf("expected 0, got %f", got)
	}
}