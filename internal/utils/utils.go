// Package utils provides common utility functions for the agent-flow project.
package utils

import "strings"

// Truncate truncates a string to the specified length.
// If the string is shorter than length, it returns the original string.
// If length is 0 or negative, it returns an empty string.
func Truncate(s string, length int) string {
	if length <= 0 {
		return ""
	}
	if len(s) <= length {
		return s
	}
	return s[:length]
}

// SplitString splits a string by a separator and returns a slice of strings.
// It handles empty separators by returning the original string as a single element.
func SplitString(s, sep string) []string {
	if sep == "" {
		return []string{s}
	}
	return strings.Split(s, sep)
}

// JoinString joins a slice of strings with a separator.
// It handles nil or empty slices by returning an empty string.
func JoinString(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, sep)
}
