package utils

import (
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		length   int
		expected string
	}{
		{"truncate_normal", "hello world", 5, "hello"},
		{"truncate_short", "hi", 10, "hi"},
		{"truncate_zero", "hello", 0, ""},
		{"truncate_negative", "hello", -1, ""},
		{"truncate_exact", "hello", 5, "hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Truncate(tt.input, tt.length)
			if result != tt.expected {
				t.Errorf("Truncate(%q, %d) = %q; want %q", tt.input, tt.length, result, tt.expected)
			}
		})
	}
}

func TestSplitString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sep      string
		expected []string
	}{
		{"split_normal", "hello,world,test", ",", []string{"hello", "world", "test"}},
		{"split_empty", "", ",", []string{""}},
		{"split_empty_sep", "hello", "", []string{"hello"}},
		{"split_single", "hello", ",", []string{"hello"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitString(tt.input, tt.sep)
			if len(result) != len(tt.expected) {
				t.Errorf("SplitString(%q, %q) length = %d; want %d", tt.input, tt.sep, len(result), len(tt.expected))
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("SplitString(%q, %q)[%d] = %q; want %q", tt.input, tt.sep, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestJoinString(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		sep      string
		expected string
	}{
		{"join_normal", []string{"hello", "world", "test"}, ",", "hello,world,test"},
		{"join_empty", []string{}, ",", ""},
		{"join_nil", nil, ",", ""},
		{"join_single", []string{"hello"}, ",", "hello"},
		{"join_no_sep", []string{"hello", "world"}, "", "helloworld"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinString(tt.input, tt.sep)
			if result != tt.expected {
				t.Errorf("JoinString(%v, %q) = %q; want %q", tt.input, tt.sep, result, tt.expected)
			}
		})
	}
}
