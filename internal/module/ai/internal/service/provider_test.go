package service

import (
	"testing"
)

func TestNormalizeOpenAIEndpoint(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"https://api.example.com", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/v1", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/v1/", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/v1/chat/completions", "https://api.example.com/v1/chat/completions"},
		{"https://api.example.com/v2", "https://api.example.com/v2/chat/completions"},
	}
	for _, test := range cases {
		actual, err := normalizeOpenAIEndpoint(test.input)
		if err != nil {
			t.Fatalf("normalizeOpenAIEndpoint(%q) error = %v", test.input, err)
		}
		if actual != test.expected {
			t.Fatalf("normalizeOpenAIEndpoint(%q) = %q, want %q", test.input, actual, test.expected)
		}
	}
}
