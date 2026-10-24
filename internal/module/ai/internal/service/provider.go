package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	aidto "cs-im-server/internal/module/ai/dto"
	"cs-im-server/internal/module/ai/internal/model"

	"cs-im-server/pkg/apperr"
	"vane-common/pkg/dataaccess"
)

const (
	defaultAIPriority = 100
	defaultAITimeout  = 30 * time.Second
)

type ProviderService struct {
	accessor dataaccess.DataAccessor[*model.AIProvider]
	client   *http.Client
	caller   ProviderCaller
}

type ProviderCaller interface {
	Generate(ctx context.Context, provider *model.AIProvider, req aidto.GenerateRequest) (aidto.GenerateResult, error)
}

type OpenAICaller struct {
	client *http.Client
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func NewProviderService() *ProviderService {
	client := &http.Client{Timeout: defaultAITimeout}
	caller := &OpenAICaller{client: client}
	return NewProviderServiceWithCaller(caller, client)
}

func NewProviderServiceWithCaller(caller ProviderCaller, client *http.Client) *ProviderService {
	if caller == nil {
		caller = &OpenAICaller{client: client}
	}
	if client == nil {
		client = &http.Client{Timeout: defaultAITimeout}
	}
	return &ProviderService{
		accessor: dataaccess.DataAccessor[*model.AIProvider]{},
		client:   client,
		caller:   caller,
	}
}

func normalizeOpenAIEndpoint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", apperr.BadRequest("ai provider endpoint is required")
	}
	endpoint, err := url.Parse(raw)
	if err != nil {
		return "", apperr.BadRequest("ai provider endpoint is invalid")
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return "", apperr.BadRequest("ai provider endpoint must use http or https")
	}
	path := strings.Trim(endpoint.Path, "/")
	path = strings.TrimSuffix(path, "/chat/completions")
	segments := strings.Split(path, "/")
	hasVersion := path != ""
	if hasVersion {
		version := segments[len(segments)-1]
		hasVersion = strings.HasPrefix(version, "v") && len(version) > 1 && version[1] >= '0' && version[1] <= '9'
	}
	if !hasVersion {
		path += "/v1"
	}
	return endpoint.Scheme + "://" + endpoint.Host + "/" + strings.Trim(path, "/") + "/chat/completions", nil
}

func (s *ProviderService) Load() error {
	providers, err := s.accessor.List(context.Background())
	if err != nil {
		return err
	}
	SetGlobalService(s)
	_ = providers
	return nil
}

func (s *ProviderService) ListProviders(ctx context.Context) []model.AIProvider {
	providers, err := s.accessor.List(ctx)
	if err != nil {
		return []model.AIProvider{}
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Priority > providers[j].Priority })
	result := make([]model.AIProvider, 0, len(providers))
	for _, item := range providers {
		result = append(result, *item)
	}
	return result
}

func (s *ProviderService) HasActiveProvider(ctx context.Context) bool {
	for _, provider := range s.ListProviders(ctx) {
		if provider.Status == "active" {
			return true
		}
	}
	return false
}

func (s *ProviderService) CreateProvider(_ context.Context, req aidto.CreateProviderRequest) (model.AIProvider, error) {
	if req.ProviderKey == "" {
		return model.AIProvider{}, apperr.BadRequest("providerKey is required")
	}
	if req.Name == "" {
		return model.AIProvider{}, apperr.BadRequest("name is required")
	}
	if req.Kind == "" {
		return model.AIProvider{}, apperr.BadRequest("kind is required")
	}
	if req.Endpoint == "" {
		return model.AIProvider{}, apperr.BadRequest("endpoint is required")
	}
	if req.Model == "" {
		return model.AIProvider{}, apperr.BadRequest("model is required")
	}
	priority := req.Priority
	if priority == 0 {
		priority = defaultAIPriority
	}
	status := "active"
	if !req.IsActive {
		status = "disabled"
	}
	item := &model.AIProvider{
		ProviderKey: req.ProviderKey,
		Name:        req.Name,
		Kind:        req.Kind,
		Endpoint:    req.Endpoint,
		APIKeyEnv:   req.APIKeyEnv,
		Model:       req.Model,
		Priority:    priority,
		Status:      status,
	}
	if err := s.accessor.Create(context.Background(), item); err != nil {
		return model.AIProvider{}, err
	}
	return *item, nil
}

func (s *ProviderService) UpdateProvider(_ context.Context, providerKey string, req aidto.UpdateProviderRequest) (model.AIProvider, error) {
	item, err := s.GetProvider(context.Background(), providerKey)
	if err != nil {
		return model.AIProvider{}, err
	}
	if req.Name != "" {
		item.Name = req.Name
	}
	if req.Endpoint != "" {
		item.Endpoint = req.Endpoint
	}
	if req.APIKeyEnv != "" {
		item.APIKeyEnv = req.APIKeyEnv
	}
	if req.Model != "" {
		item.Model = req.Model
	}
	if req.Priority > 0 {
		item.Priority = req.Priority
	}
	if err := s.accessor.Update(context.Background(), &item); err != nil {
		return model.AIProvider{}, err
	}
	return item, nil
}

func (s *ProviderService) SetProviderStatus(_ context.Context, providerKey string, active bool) (model.AIProvider, error) {
	item, err := s.GetProvider(context.Background(), providerKey)
	if err != nil {
		return model.AIProvider{}, err
	}
	item.Status = "disabled"
	if active {
		item.Status = "active"
	}
	if err := s.accessor.Update(context.Background(), &item); err != nil {
		return model.AIProvider{}, err
	}
	return item, nil
}

func (s *ProviderService) GetProvider(_ context.Context, providerKey string) (model.AIProvider, error) {
	item, err := s.accessor.First(context.Background(), dataaccess.Eq("provider_key", providerKey))
	if err != nil {
		return model.AIProvider{}, apperr.NotFound("ai provider %s not found", providerKey)
	}
	return *item, nil
}

func (s *ProviderService) Generate(ctx context.Context, req aidto.GenerateRequest) (aidto.GenerateResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return aidto.GenerateResult{}, apperr.BadRequest("prompt is required")
	}
	providerKey := strings.TrimSpace(req.ProviderID)
	if providerKey == "" {
		provider, err := s.pickProvider(ctx, req.ProviderID)
		if err != nil {
			return aidto.GenerateResult{}, err
		}
		providerKey = provider.ProviderKey
	}
	provider, err := s.GetProvider(ctx, providerKey)
	if err != nil {
		return aidto.GenerateResult{}, err
	}
	if provider.Status != "active" {
		return aidto.GenerateResult{}, apperr.Forbidden("ai provider %s is disabled", provider.ProviderKey)
	}
	result, err := s.caller.Generate(ctx, &provider, req)
	if err != nil {
		return aidto.GenerateResult{}, err
	}
	return result, nil
}

func (s *ProviderService) pickProvider(_ context.Context, providerID string) (model.AIProvider, error) {
	providers := s.ListProviders(context.Background())
	if len(providers) == 0 {
		return model.AIProvider{}, apperr.NotFound("ai provider not configured")
	}
	for _, provider := range providers {
		if providerID == "" || provider.ProviderKey == providerID {
			if provider.Status == "active" {
				return provider, nil
			}
		}
	}
	return model.AIProvider{}, apperr.NotFound("ai provider not found")
}

func (s *OpenAICaller) Generate(ctx context.Context, provider *model.AIProvider, req aidto.GenerateRequest) (aidto.GenerateResult, error) {
	if provider == nil {
		return aidto.GenerateResult{}, apperr.BadRequest("ai provider missing")
	}
	messages := make([]openAIMessage, 0, 2)
	if req.System != "" {
		messages = append(messages, openAIMessage{Role: "system", Content: req.System})
	}
	messages = append(messages, openAIMessage{Role: "user", Content: req.Prompt})
	payload, err := json.Marshal(openAIRequest{Model: provider.Model, Messages: messages})
	if err != nil {
		return aidto.GenerateResult{}, err
	}
	endpoint, err := normalizeOpenAIEndpoint(provider.Endpoint)
	if err != nil {
		return aidto.GenerateResult{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(payload))
	if err != nil {
		return aidto.GenerateResult{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if provider.APIKeyEnv != "" && os.Getenv(provider.APIKeyEnv) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+os.Getenv(provider.APIKeyEnv))
	}
	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		return aidto.GenerateResult{}, err
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		body, _ := io.ReadAll(httpResp.Body)
		return aidto.GenerateResult{}, fmt.Errorf("ai provider %s returned status %d: %s", provider.ProviderKey, httpResp.StatusCode, string(body))
	}
	var response openAIResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
		return aidto.GenerateResult{}, err
	}
	if len(response.Choices) == 0 {
		return aidto.GenerateResult{}, apperr.New(502, "AI_PROVIDER_ERROR", "ai provider returned no choices")
	}
	return aidto.GenerateResult{
		Text:         response.Choices[0].Message.Content,
		ProviderID:   provider.ProviderKey,
		ProviderKind: provider.Kind,
		ModelID:      provider.Model,
		Priority:     provider.Priority,
	}, nil
}
