package log

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"INFO":    slog.LevelInfo,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"":        slog.LevelInfo,
		"verbose": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Fatalf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestComponentStructuredOutput(t *testing.T) {
	var buf bytes.Buffer
	Init(Options{JSON: true, Level: slog.LevelInfo, Output: &buf})

	Component("worker").Info("output saved", "path", "/tmp/out.txt", "bytes", 42)

	out := buf.String()
	for _, key := range []string{`"component":"worker"`, `"msg":"output saved"`, `"path":"/tmp/out.txt"`, `"bytes":42`} {
		if !strings.Contains(out, key) {
			t.Fatalf("expected %q in log output, got: %s", key, out)
		}
	}
}