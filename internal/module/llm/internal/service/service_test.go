package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs-im-server/internal/module/llm/internal/config"
)

func testConfig(t *testing.T, endpoint, provider, model string) config.Config {
	t.Helper()
	const secretName = "CS_IM_TEST_LLM_SECRET"
	t.Setenv(secretName, "test-secret")
	path := filepath.Join(t.TempDir(), "llm.yaml")
	contents := strings.Join([]string{
		"enabled: true",
		"max_request_bytes: 1048576",
		"max_response_bytes: 1048576",
		"models:",
		"  public-selector:",
		"    provider: " + provider,
		"    endpoint: " + endpoint,
		"    model: " + model,
		"    api_key_env: " + secretName,
		"    timeout_seconds: 5",
		"    anthropic_version: 2023-06-01",
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

type requestRewriteTransport struct{ destination string }

func (t requestRewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	target, err := url.Parse(t.destination)
	if err != nil {
		return nil, err
	}
	clone.URL.Scheme, clone.URL.Host, clone.Host = target.Scheme, target.Host, target.Host
	return http.DefaultTransport.RoundTrip(clone)
}

func upstreamClient(server *httptest.Server) *http.Client {
	return &http.Client{Transport: requestRewriteTransport{destination: server.URL}}
}

func TestChatUsesConfiguredModelAndPreservesTools(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-secret" {
			t.Errorf("Authorization = %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"model":"MiniMax-M3"`) {
			t.Errorf("configured model was not used: %s", body)
		}
		if !strings.Contains(string(body), `"tools"`) {
			t.Errorf("tools field was not preserved: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"<think>hidden</think>answer"}}]}`)
	}))
	defer upstream.Close()
	cfg := testConfig(t, "https://api.example.test/v1", "minimax", "MiniMax-M3")
	svc := New(cfg, upstreamClient(upstream))
	response, err := svc.Chat(context.Background(), "public-selector", []byte(`{"model":"untrusted","messages":[{"role":"user","content":"hi"}],"tools":[]}`), "")
	if err != nil {
		t.Fatal(err)
	}
	response, err = svc.WrapMiniMaxResponse("public-selector", response)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ReadBody(response, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "<think>") || strings.Contains(string(body), "hidden") || !strings.Contains(string(body), `"content":"answer"`) {
		t.Fatalf("MiniMax response normalization failed: %s", body)
	}
}

func TestAnthropicUsesNativeHeadersAndMessagesShape(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != "test-secret" {
			t.Error("X-Api-Key missing")
		}
		if r.Header.Get("Anthropic-Version") != "2023-06-01" {
			t.Error("Anthropic-Version missing")
		}
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `"system":"system prompt"`) {
			t.Errorf("Anthropic system field missing: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"msg_test","content":[{"type":"text","text":"answer"}]}`)
	}))
	defer upstream.Close()
	cfg := testConfig(t, "https://api.example.test/v1", "anthropic", "claude-test")
	svc := New(cfg, upstreamClient(upstream))
	response, err := svc.Messages(context.Background(), "public-selector", []byte(`{"model":"untrusted","max_tokens":64,"system":"system prompt","messages":[{"role":"user","content":"hi"}]}`), "")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
}

func TestOpenCodeHeadersDoNotLeakToOtherProviders(t *testing.T) {
	if got := OpenCodeHeaders("https://opencode.ai/zen/v1", "session-123", true).Get("X-OpenCode-Session"); got != "session-123" {
		t.Fatalf("OpenCode session = %q", got)
	}
	if got := OpenCodeHeaders("https://api.openai.com/v1", "session-123", true).Get("X-OpenCode-Session"); got != "" {
		t.Fatalf("session header leaked: %q", got)
	}
}

func TestOpenCodeGoEndpointRouting(t *testing.T) {
	for input, want := range map[string]string{
		"https://opencode.ai/zen/go/v1": "https://opencode.ai/zen/go/v1/chat/completions",
		"https://opencode.ai/zen/v1":    "https://opencode.ai/zen/v1/chat/completions",
	} {
		got, err := appendEndpointPath(input, "/chat/completions")
		if err != nil || got != want {
			t.Fatalf("appendEndpointPath(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
}

func TestOpenCodeEndpointRewritesSessionHeader(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/zen/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("X-OpenCode-Session"); got != "session-123" {
			t.Errorf("X-OpenCode-Session = %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != "cs-im-server/1.0" {
			t.Errorf("User-Agent = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[]}`)
	}))
	defer upstream.Close()
	cfg := testConfig(t, "https://opencode.ai/zen/v1", "opencode", "example/model")
	model := cfg.Models["public-selector"]
	model.OpenCodeSessionHeader = true
	cfg.Models["public-selector"] = model
	svc := New(cfg, upstreamClient(upstream))
	response, err := svc.Chat(context.Background(), "public-selector", []byte(`{"model":"public-selector","messages":[{"role":"user","content":"hi"}]}`), "session-123")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
}

func TestTranslationRequiresExplicitDefaultModel(t *testing.T) {
	svc := New(config.Config{
		Enabled: true,
		Models: map[string]config.ModelConfig{
			"available-chat": {Provider: "openai", Model: "gpt-test"},
		},
	}, nil)
	if got := svc.TranslationSelector(); got != "" {
		t.Fatalf("translation selector = %q; expected no implicit selection", got)
	}
}

func TestCleanMiniMaxContentSeparatesReasoning(t *testing.T) {
	content, reasoning := CleanMiniMaxContent("<think>private</think>answer<analysis>also private</analysis>")
	if content != "answer" {
		t.Fatalf("content = %q", content)
	}
	if !strings.Contains(reasoning, "private") || !strings.Contains(reasoning, "also private") {
		t.Fatalf("reasoning = %q", reasoning)
	}
}

func TestMiniMaxSSESeparatesTagsSplitAcrossChunks(t *testing.T) {
	input := "event: message\n"
	input += "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"<thi\"}}]}\n\n"
	input += "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"nk>private reasoning</think>answer\"}}]}\n\n"
	input += "data: [DONE]\n\n"
	body := newMiniMaxSSEBody(io.NopCloser(strings.NewReader(input)))
	filtered, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(filtered)
	if strings.Contains(text, "<think>") || strings.Contains(text, "</think>") {
		t.Fatalf("reasoning tags leaked into SSE: %s", text)
	}
	if !strings.Contains(text, `"content":"answer"`) {
		t.Fatalf("final content missing: %s", text)
	}
	if strings.Contains(text, "private reasoning") {
		t.Fatalf("thinking content leaked: %s", text)
	}
}

func TestMiniMaxSSEPreservesSmallSafeEventAndRemovesPrivateShortEvent(t *testing.T) {
	input := "data: {\"id\":\"chatcmpl-test\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi there, this is enough safe output to flush the filter.\"}}]}\n\n"
	input += "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"<think>hidden</think>\"}}]}\n\n"
	body := newMiniMaxSSEBody(io.NopCloser(strings.NewReader(input)))
	filtered, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(filtered)
	if strings.Contains(text, "<think>") || strings.Contains(text, "</think>") || strings.Contains(text, "hidden") {
		t.Fatalf("reasoning tags were not separated: %s", filtered)
	}
	if !strings.Contains(text, "chatcmpl-test") || !strings.Contains(text, "enough safe output") || strings.Contains(text, "hidden") {
		t.Fatalf("safe event fields or content were not preserved: %s", filtered)
	}
}

func TestMiniMaxAnthropicSSEKeepsAnthropicEventNames(t *testing.T) {
	input := strings.Join([]string{
		"event: content_block_start",
		"data: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}",
		"",
		"event: content_block_delta",
		"data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"<think>private</think>answer\"}}",
		"",
	}, "\n")
	body := newMiniMaxAnthropicSSEBody(io.NopCloser(strings.NewReader(input)))
	filtered, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	text := string(filtered)
	if !strings.Contains(text, "event: content_block_delta") || strings.Contains(text, "private") || !strings.Contains(text, `"text":"answer"`) {
		t.Fatalf("Anthropic MiniMax stream was not normalized in Anthropic format: %s", text)
	}
}
