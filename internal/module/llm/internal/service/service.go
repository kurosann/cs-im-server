// Package service owns provider selection, protocol adapters, bounded I/O, and
// response handling behind one interface.
package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"cs-im-server/internal/module/llm/internal/config"
)

var (
	ErrDisabled       = errors.New("llm gateway disabled")
	ErrModelNotFound  = errors.New("llm model not found")
	ErrInvalidRequest = errors.New("invalid llm request")
	ErrUpstream       = errors.New("llm upstream unavailable")
	ErrTooLarge       = errors.New("llm payload exceeds configured limit")
)

var reasoningTagPattern = regexp.MustCompile(`(?is)</?\s*(think|thinking|analysis|reasoning)\b[^>]*>`)

type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

type Service struct {
	cfg    config.Config
	client *http.Client
}

func (s *Service) MaxRequestBytes() int64 { return s.cfg.MaxRequestBytes }

func New(cfg config.Config, client *http.Client) *Service {
	if client == nil {
		client = &http.Client{}
	}
	clientCopy := *client
	if clientCopy.CheckRedirect == nil {
		clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	}
	return &Service{cfg: cfg, client: &clientCopy}
}

func (s *Service) ListModels() ([]ModelInfo, error) {
	if !s.cfg.Enabled {
		return nil, ErrDisabled
	}
	selectors := s.cfg.Selectors()
	models := make([]ModelInfo, 0, len(selectors))
	for _, selector := range selectors {
		models = append(models, ModelInfo{ID: selector, Object: "model", OwnedBy: s.cfg.Models[selector].Provider})
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models, nil
}

func (s *Service) TranslationModel() string {
	if !s.cfg.Enabled {
		return ""
	}
	return s.cfg.DefaultTranslationModel
}

func (s *Service) TranslationSelector() string {
	return s.TranslationModel()
}

func (s *Service) Translate(ctx context.Context, text string, sourceLang string, targetLang string) (string, error) {
	selector := s.TranslationModel()
	if selector == "" {
		return "", ErrModelNotFound
	}
	var messages []map[string]string
	if strings.TrimSpace(sourceLang) == "" || sourceLang == "auto" {
		sourceLang = "auto"
	}
	if strings.TrimSpace(targetLang) == "" || targetLang == "auto" {
		targetLang = oppositeLanguage(text, sourceLang)
	}
	prompt := "Translate the user text from " + sourceLang + " to " + targetLang + ". Preserve formatting and output only the translation."
	model := s.cfg.Models[selector]
	if config.Protocol(model.Provider) == "anthropic" {
		body, _ := json.Marshal(map[string]any{"model": selector, "max_tokens": model.MaxTokens, "system": prompt, "messages": []map[string]string{{"role": "user", "content": text}}, "stream": false})
		response, err := s.Messages(ctx, selector, body, "")
		if err != nil {
			return "", err
		}
		defer response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return "", fmt.Errorf("%w: Anthropic translation status %d", ErrUpstream, response.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, s.cfg.MaxResponseBytes+1))
		if err != nil {
			return "", fmt.Errorf("%w: read Anthropic translation response", ErrUpstream)
		}
		if int64(len(data)) > s.cfg.MaxResponseBytes {
			return "", ErrTooLarge
		}
		var result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if err := json.Unmarshal(data, &result); err != nil {
			return "", fmt.Errorf("%w: invalid Anthropic translation response", ErrUpstream)
		}
		var output strings.Builder
		for _, block := range result.Content {
			if block.Type == "text" {
				output.WriteString(block.Text)
			}
		}
		translated := strings.TrimSpace(output.String())
		if strings.TrimSpace(translated) == "" {
			return "", fmt.Errorf("%w: empty translation", ErrUpstream)
		}
		return strings.TrimSpace(translated), nil
	}
	messages = []map[string]string{{"role": "system", "content": prompt}, {"role": "user", "content": text}}
	body, _ := json.Marshal(map[string]any{"model": selector, "messages": messages, "stream": false})
	response, err := s.Chat(ctx, selector, body, "")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("%w: translation upstream returned status %d", ErrUpstream, response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, s.cfg.MaxResponseBytes+1))
	if err != nil {
		return "", fmt.Errorf("%w: read translation response", ErrUpstream)
	}
	if int64(len(data)) > s.cfg.MaxResponseBytes {
		return "", ErrTooLarge
	}
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &result); err != nil || len(result.Choices) == 0 {
		return "", fmt.Errorf("%w: invalid translation response", ErrUpstream)
	}
	content, _ := CleanMiniMaxContent(result.Choices[0].Message.Content)
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("%w: empty translation", ErrUpstream)
	}
	return strings.TrimSpace(content), nil
}

func oppositeLanguage(text string, sourceLang string) string {
	if sourceLang != "" && sourceLang != "auto" {
		if strings.HasPrefix(strings.ToLower(sourceLang), "zh") {
			return "en"
		}
		return "zh-CN"
	}
	for _, char := range text {
		if char >= 0x3400 && char <= 0x9fff {
			return "en"
		}
	}
	return "zh-CN"
}

func (s *Service) Chat(ctx context.Context, selector string, raw []byte, sessionID string) (*http.Response, error) {
	model, err := s.model(selector, raw)
	if err != nil {
		return nil, err
	}
	if config.Protocol(model.Provider) != "openai" {
		return nil, fmt.Errorf("%w: selected model requires Anthropic Messages protocol", ErrInvalidRequest)
	}
	payload, err := prepareOpenAIRequest(raw, model)
	if err != nil {
		return nil, err
	}
	endpoint, err := appendEndpointPath(model.Endpoint, "/chat/completions")
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%w: build request", ErrInvalidRequest)
	}
	request.Header.Set("Content-Type", "application/json")
	if payloadWantsStream(payload) {
		request.Header.Set("Accept", "text/event-stream")
	} else {
		request.Header.Set("Accept", "application/json")
	}
	request.Header.Set("Authorization", "Bearer "+config.Secret(model))
	for key, values := range OpenCodeHeaders(model.Endpoint, sessionID, model.OpenCodeSessionHeader) {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	response, err := s.clientFor(model).Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	return s.boundResponse(response, model)
}

func (s *Service) Messages(ctx context.Context, selector string, raw []byte, sessionID string) (*http.Response, error) {
	model, err := s.model(selector, raw)
	if err != nil {
		return nil, err
	}
	if config.Protocol(model.Provider) != "anthropic" {
		return nil, fmt.Errorf("%w: selected model requires OpenAI Chat Completions protocol", ErrInvalidRequest)
	}
	payload, err := prepareAnthropicRequest(raw, model)
	if err != nil {
		return nil, err
	}
	endpoint, err := appendEndpointPath(model.Endpoint, "/messages")
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("%w: build request", ErrInvalidRequest)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("X-Api-Key", config.Secret(model))
	request.Header.Set("Anthropic-Version", model.AnthropicVersion)
	for key, values := range OpenCodeHeaders(model.Endpoint, sessionID, model.OpenCodeSessionHeader) {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	response, err := s.clientFor(model).Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	response, err = s.boundResponse(response, model)
	if err == nil && MiniMaxModel(model.Provider, model.Model, model.Endpoint) {
		if isEventStream(response.Header.Get("Content-Type")) {
			response.Body = newMiniMaxAnthropicSSEBody(response.Body)
		} else {
			response, err = normalizeMiniMaxAnthropicResponse(response, s.cfg.MaxResponseBytes)
		}
	}
	return response, err
}

func (s *Service) model(selector string, raw []byte) (config.ModelConfig, error) {
	if !s.cfg.Enabled {
		return config.ModelConfig{}, ErrDisabled
	}
	if len(raw) == 0 {
		return config.ModelConfig{}, fmt.Errorf("%w: request body is required", ErrInvalidRequest)
	}
	if int64(len(raw)) > s.cfg.MaxRequestBytes {
		return config.ModelConfig{}, ErrTooLarge
	}
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return config.ModelConfig{}, fmt.Errorf("%w: model selector is required", ErrInvalidRequest)
	}
	model, ok := s.cfg.Models[selector]
	if !ok || model.Disabled {
		return config.ModelConfig{}, ErrModelNotFound
	}
	return model, nil
}

func (s *Service) clientFor(model config.ModelConfig) *http.Client {
	client := *s.client
	client.Timeout = model.Timeout
	return &client
}

func (s *Service) boundResponse(response *http.Response, model config.ModelConfig) (*http.Response, error) {
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		limited, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		_ = response.Body.Close()
		response.Body = io.NopCloser(strings.NewReader(sanitizeUpstreamError(limited, config.Protocol(model.Provider))))
		return response, nil
	}
	response.Body = newBoundedReadCloser(response.Body, s.cfg.MaxResponseBytes)
	return response, nil
}

func (s *Service) WrapMiniMaxStream(selector string, response *http.Response) *http.Response {
	model, ok := s.cfg.Models[strings.TrimSpace(selector)]
	if !ok || !MiniMaxModel(model.Provider, model.Model, model.Endpoint) || !isEventStream(response.Header.Get("Content-Type")) {
		return response
	}
	response.Body = newMiniMaxSSEBody(response.Body)
	return response
}

func (s *Service) WrapMiniMaxResponse(selector string, response *http.Response) (*http.Response, error) {
	model, ok := s.cfg.Models[strings.TrimSpace(selector)]
	if !ok || !MiniMaxModel(model.Provider, model.Model, model.Endpoint) || isEventStream(response.Header.Get("Content-Type")) {
		return response, nil
	}
	body, err := ReadBody(response, s.cfg.MaxResponseBytes)
	if err != nil {
		return nil, err
	}
	body = NormalizeMiniMaxResponse(body)
	response.Body = io.NopCloser(bytes.NewReader(body))
	response.ContentLength = int64(len(body))
	return response, nil
}

func prepareOpenAIRequest(raw []byte, model config.ModelConfig) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return nil, fmt.Errorf("%w: body must be a JSON object", ErrInvalidRequest)
	}
	if err := validateMessages(payload["messages"]); err != nil {
		return nil, err
	}
	modelJSON, _ := json.Marshal(model.Model)
	payload["model"] = modelJSON
	if err := clampTokenField(payload, "max_tokens", model.MaxTokens); err != nil {
		return nil, err
	}
	if err := clampTokenField(payload, "max_completion_tokens", model.MaxTokens); err != nil {
		return nil, err
	}
	if model.MaxTokens > 0 {
		if _, hasMaxTokens := payload["max_tokens"]; !hasMaxTokens {
			if _, hasMaxCompletion := payload["max_completion_tokens"]; !hasMaxCompletion {
				value, _ := json.Marshal(model.MaxTokens)
				payload["max_tokens"] = value
			}
		}
	}
	if model.Temperature != nil {
		if _, exists := payload["temperature"]; !exists {
			value, _ := json.Marshal(*model.Temperature)
			payload["temperature"] = value
		}
	}
	return json.Marshal(payload)
}

func prepareAnthropicRequest(raw []byte, model config.ModelConfig) ([]byte, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil || payload == nil {
		return nil, fmt.Errorf("%w: body must be a JSON object", ErrInvalidRequest)
	}
	if err := validateMessages(payload["messages"]); err != nil {
		return nil, err
	}
	modelJSON, _ := json.Marshal(model.Model)
	payload["model"] = modelJSON
	if err := clampTokenField(payload, "max_tokens", model.MaxTokens); err != nil {
		return nil, err
	}
	if _, exists := payload["max_tokens"]; !exists {
		value, _ := json.Marshal(model.MaxTokens)
		payload["max_tokens"] = value
	}
	return json.Marshal(payload)
}

func clampTokenField(payload map[string]json.RawMessage, key string, maximum int) error {
	raw, exists := payload[key]
	if !exists {
		return nil
	}
	var requested int
	if err := json.Unmarshal(raw, &requested); err != nil || requested < 1 {
		return fmt.Errorf("%w: %s must be a positive integer", ErrInvalidRequest, key)
	}
	if maximum > 0 && requested > maximum {
		bounded, _ := json.Marshal(maximum)
		payload[key] = bounded
	}
	return nil
}

func appendEndpointPath(endpoint, suffix string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" || parsed.User != nil || parsed.Scheme != "https" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%w: invalid configured HTTPS endpoint", ErrInvalidRequest)
	}
	path := strings.TrimRight(parsed.Path, "/")
	if strings.HasSuffix(path, suffix) {
		return parsed.String(), nil
	}
	if suffix == "/chat/completions" && strings.HasSuffix(path, "/api/v3") {
		parsed.Path = path + suffix
		return parsed.String(), nil
	}
	if suffix == "/messages" && !strings.HasSuffix(path, "/v1") && strings.HasSuffix(path, "/anthropic") {
		parsed.Path = path + "/v1" + suffix
		return parsed.String(), nil
	}
	if !strings.HasSuffix(path, "/v1") {
		path += "/v1"
	}
	parsed.Path = path + suffix
	return parsed.String(), nil
}

func validateMessages(raw json.RawMessage) error {
	var messages []json.RawMessage
	if len(raw) == 0 || json.Unmarshal(raw, &messages) != nil || len(messages) == 0 {
		return fmt.Errorf("%w: messages must be a non-empty array", ErrInvalidRequest)
	}
	return nil
}

func payloadWantsStream(data []byte) bool {
	var payload map[string]json.RawMessage
	if json.Unmarshal(data, &payload) != nil {
		return false
	}
	var value bool
	_ = json.Unmarshal(payload["stream"], &value)
	return value
}
func isEventStream(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/event-stream")
}

func OpenCodeHeaders(endpoint, sessionID string, enabled bool) http.Header {
	header := make(http.Header)
	parsed, err := url.Parse(endpoint)
	if err != nil || !strings.EqualFold(parsed.Hostname(), "opencode.ai") {
		return header
	}
	if enabled && strings.TrimSpace(sessionID) != "" {
		header.Set("X-OpenCode-Session", strings.TrimSpace(sessionID))
	}
	header.Set("User-Agent", "cs-im-server/1.0")
	return header
}

func sanitizeUpstreamError(body []byte, protocol string) string {
	_ = body
	return nativeErrorBody(protocol, "upstream request failed")
}

func nativeErrorBody(protocol, message string) string {
	var payload map[string]any
	if protocol == "anthropic" {
		payload = map[string]any{"type": "error", "error": map[string]string{"type": "api_error", "message": message}}
	} else {
		payload = map[string]any{"error": map[string]string{"message": message}}
	}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

type boundedReadCloser struct {
	reader io.Reader
	closer io.Closer
	limit  int64
	read   int64
}

func newBoundedReadCloser(body io.ReadCloser, limit int64) io.ReadCloser {
	return &boundedReadCloser{reader: body, closer: body, limit: limit}
}
func (r *boundedReadCloser) Read(p []byte) (int, error) {
	if r.read >= r.limit {
		var extra [1]byte
		n, err := r.reader.Read(extra[:])
		if n > 0 {
			return 0, ErrTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > r.limit-r.read {
		p = p[:r.limit-r.read]
	}
	n, err := r.reader.Read(p)
	r.read += int64(n)
	return n, err
}
func (r *boundedReadCloser) Close() error { return r.closer.Close() }

func ReadBody(response *http.Response, limit int64) ([]byte, error) {
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("%w: read response", ErrUpstream)
	}
	if int64(len(body)) > limit {
		return nil, ErrTooLarge
	}
	return body, nil
}

func NormalizeMiniMaxResponse(body []byte) []byte {
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return body
	}
	choices, _ := payload["choices"].([]any)
	changed := false
	for _, raw := range choices {
		choice, _ := raw.(map[string]any)
		message, _ := choice["message"].(map[string]any)
		content, _ := message["content"].(string)
		if message == nil || content == "" {
			continue
		}
		cleaned, reasoning := CleanMiniMaxContent(content)
		if cleaned != content {
			message["content"] = cleaned
			changed = true
		}
		if reasoning != "" {
			if _, exists := message["reasoning_content"]; exists {
				delete(message, "reasoning_content")
				changed = true
			}
		}
	}
	if !changed {
		return body
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return body
	}
	return encoded
}

func CleanMiniMaxContent(content string) (string, string) {
	var reasoning strings.Builder
	var answer strings.Builder
	inReasoning := false
	for len(content) > 0 {
		match := reasoningTagPattern.FindStringIndex(content)
		if match == nil {
			if inReasoning {
				reasoning.WriteString(content)
			} else {
				answer.WriteString(content)
			}
			break
		}
		before, tag := content[:match[0]], content[match[0]:match[1]]
		if inReasoning {
			reasoning.WriteString(before)
		} else {
			answer.WriteString(before)
		}
		closing := strings.HasPrefix(strings.TrimSpace(tag), "</")
		if closing {
			inReasoning = false
		} else if !inReasoning {
			inReasoning = true
		}
		content = content[match[1]:]
	}
	return strings.TrimSpace(answer.String()), strings.TrimSpace(reasoning.String())
}

func normalizeMiniMaxAnthropicResponse(response *http.Response, limit int64) (*http.Response, error) {
	body, err := ReadBody(response, limit)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return responseWithBody(response, body), nil
	}
	blocks, _ := payload["content"].([]any)
	changed := false
	for _, raw := range blocks {
		block, _ := raw.(map[string]any)
		if block == nil || block["type"] != "text" {
			continue
		}
		text, _ := block["text"].(string)
		cleaned, _ := CleanMiniMaxContent(text)
		if cleaned != text {
			block["text"] = cleaned
			changed = true
		}
	}
	if !changed {
		return responseWithBody(response, body), nil
	}
	normalized, err := json.Marshal(payload)
	if err != nil {
		return responseWithBody(response, body), nil
	}
	return responseWithBody(response, normalized), nil
}

func responseWithBody(response *http.Response, body []byte) *http.Response {
	response.Body = io.NopCloser(bytes.NewReader(body))
	response.ContentLength = int64(len(body))
	return response
}

func MiniMaxModel(provider, model, endpoint string) bool {
	return provider == "minimax" || strings.Contains(strings.ToLower(model), "minimax") || strings.Contains(strings.ToLower(endpoint), "minimaxi")
}

func SessionID(value string) string { return strings.TrimSpace(value) }
