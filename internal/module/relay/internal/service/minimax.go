package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"cs-im-server/internal/cfg"
)

const (
	aiReplyTemperature = 0.2
	aiReplyMaxTokens   = 300
)

var errAINotConfigured = errors.New("未配置 CS_IM_AI_API_KEY")

type Completion struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
	FinishReason     string
}

type CompletionClient interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string) (Completion, error)
}

type miniMaxClient struct {
	httpClient *http.Client
}

func newMiniMaxClient() CompletionClient {
	return miniMaxClient{httpClient: &http.Client{}}
}

func (c miniMaxClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (Completion, error) {
	config := cfg.GetConfig()
	if config == nil || strings.TrimSpace(config.AI.APIKey) == "" {
		return Completion{}, errAINotConfigured
	}
	timeout := time.Duration(config.AI.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload := map[string]any{
		"model":       config.AI.Model,
		"stream":      false,
		"temperature": aiReplyTemperature,
		"max_tokens":  aiReplyMaxTokens,
		"thinking": map[string]string{
			"type": "disabled",
		},
		"response_format": map[string]string{
			"type": "json_object",
		},
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Completion{}, err
	}
	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, resolveChatCompletionsURL(config.AI.Endpoint), bytes.NewReader(body))
	if err != nil {
		return Completion{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+config.AI.APIKey)

	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return Completion{}, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return Completion{}, err
	}
	if response.StatusCode != http.StatusOK {
		return Completion{}, fmt.Errorf("模型上游返回 HTTP %d", response.StatusCode)
	}
	return parseChatCompletion(responseBody)
}

func resolveChatCompletionsURL(endpoint string) string {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		endpoint = "https://api.minimaxi.com/v1"
	}
	if strings.HasSuffix(endpoint, "/chat/completions") {
		return endpoint
	}
	return endpoint + "/chat/completions"
}

func parseChatCompletion(body []byte) (Completion, error) {
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return Completion{}, fmt.Errorf("模型响应不是合法 JSON")
	}
	if len(response.Choices) == 0 {
		return Completion{}, fmt.Errorf("模型响应缺少 choices")
	}
	return Completion{
		Content:          strings.TrimSpace(response.Choices[0].Message.Content),
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		FinishReason:     response.Choices[0].FinishReason,
	}, nil
}

func aiModelName() string {
	config := cfg.GetConfig()
	if config == nil || strings.TrimSpace(config.AI.Model) == "" {
		return "MiniMax-M3"
	}
	return config.AI.Model
}
