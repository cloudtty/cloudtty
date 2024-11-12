package controllers

import (
	"testing"
)

func TestTruncateAndHashIfExceed(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "short-pod-name",
			expected: "short-pod-name",
		},
		{
			input:    "pod-name-with-63-characters-12345678901234567891111111123456789", // 63 characters
			expected: "pod-name-with-63-characters-12345678901234567891111111123456789",
		},
		{
			input:    "another-very-long-pod-name-that-exceeds-63-characters-123456789012345678901111111",
			expected: "another-very-long-pod-name-that5b2d51014a9e6924828703237fa40bfd",
		},
	}

	for _, test := range tests {
		result := truncateAndHashIfExceed(test.input)
		if result != test.expected {
			t.Errorf("For input '%s', expected '%s', but got '%s'", test.input, test.expected, result)
		}
	}
}
