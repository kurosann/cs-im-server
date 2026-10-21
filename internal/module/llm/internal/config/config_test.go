package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadResolvesModelSecretFromEnvironment(t *testing.T) {
	const secretName = "CS_IM_TEST_LLM_SECRET"
	if err := os.Setenv(secretName, "test-secret-value"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv(secretName) })
	path := filepath.Join(t.TempDir(), "llm.yaml")
	data := []byte("enabled: true\nmodels:\n  chat:\n    provider: minimax\n    endpoint: https://api.minimax.io/v1\n    model: MiniMax-M3\n    api_key_env: " + secretName + "\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if Secret(cfg.Models["chat"]) != "test-secret-value" {
		t.Fatal("model secret was not resolved")
	}
	if got := cfg.Selectors(); len(got) != 1 || got[0] != "chat" {
		t.Fatalf("selectors = %#v", got)
	}
}

func TestLoadRejectsMissingSecretAndHTTPUpstream(t *testing.T) {
	t.Run("missing secret", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "llm.yaml")
		data := []byte("enabled: true\nmodels:\n  chat:\n    provider: openai\n    endpoint: https://api.openai.com/v1\n    model: model-a\n    api_key_env: CS_IM_MISSING_LLM_SECRET\n")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("Load accepted missing API key environment variable")
		}
	})
	t.Run("insecure endpoint", func(t *testing.T) {
		const secretName = "CS_IM_TEST_LLM_SECRET"
		if err := os.Setenv(secretName, "test"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Unsetenv(secretName) })
		path := filepath.Join(t.TempDir(), "llm.yaml")
		data := []byte("enabled: true\nmodels:\n  chat:\n    provider: openai\n    endpoint: http://localhost:1234/v1\n    model: model-a\n    api_key_env: " + secretName + "\n")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(path); err == nil {
			t.Fatal("Load accepted HTTP upstream endpoint")
		}
	})
}

func TestOpenCodeSessionHeaderRequiresOpenCodeEndpoint(t *testing.T) {
	const secretName = "CS_IM_TEST_LLM_SECRET"
	t.Setenv(secretName, "test")
	path := filepath.Join(t.TempDir(), "llm.yaml")
	data := []byte(strings.Join([]string{
		"enabled: true", "models:", "  chat:", "    provider: openai",
		"    endpoint: https://api.openai.com/v1", "    model: model-a",
		"    api_key_env: " + secretName, "    opencode_session_header: true",
	}, "\n") + "\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("OpenCode session header accepted for a non-OpenCode endpoint")
	}
}
