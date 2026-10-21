package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	aipkg "cs-im-server/internal/module/ai/pkg/ai"
	translationdto "cs-im-server/internal/module/translation/dto"
	"cs-im-server/internal/module/translation/internal/model"
	commontranslate "vane-common/pkg/translates/libretranslate"

	"cs-im-server/pkg/apperr"
	"golang.org/x/sync/singleflight"
	"vane-common/pkg/dataaccess"
)

const (
	providerKindAI = "ai"
	providerKindMT = "mt"

	autoProviderID = "auto"
)

type TranslationService struct {
	aiService        aipkg.AIService
	providerAccessor dataaccess.DataAccessor[*model.Provider]
	cacheAccessor    dataaccess.DataAccessor[*model.Cache]
	sf               singleflight.Group
}

type Provider struct {
	ID        string
	Kind      string
	Endpoint  string
	APIKeyEnv string
	Model     string
	Priority  int
	Translate func(ctx context.Context, item translationdto.TranslateItem) (string, error)
}

type deeplxRequest struct {
	Text       string `json:"text"`
	SourceLang string `json:"source_lang"`
	TargetLang string `json:"target_lang"`
}

type deeplxResponse struct {
	Code    int    `json:"code"`
	Data    string `json:"data"`
	Message string `json:"message"`
}

func NewTranslationService(aiService aipkg.AIService) *TranslationService {
	return &TranslationService{
		aiService:        aiService,
		providerAccessor: dataaccess.DataAccessor[*model.Provider]{},
		cacheAccessor:    dataaccess.DataAccessor[*model.Cache]{},
	}
}

func (s *TranslationService) Load() error {
	return nil
}

func (s *TranslationService) CreateProvider(_ context.Context, req translationdto.CreateProviderRequest) (model.Provider, error) {
	if req.ProviderKey == "" {
		return model.Provider{}, apperr.BadRequest("providerId is required")
	}
	if req.Name == "" {
		return model.Provider{}, apperr.BadRequest("name is required")
	}
	if req.Kind == "" {
		return model.Provider{}, apperr.BadRequest("kind is required")
	}
	priority := req.Priority
	if priority == 0 {
		priority = 50
	}
	item := &model.Provider{
		ProviderKey: req.ProviderKey,
		Name:        req.Name,
		Kind:        req.Kind,
		Endpoint:    req.Endpoint,
		APIKeyEnv:   req.APIKeyEnv,
		Model:       req.Model,
		Priority:    priority,
		Status:      "active",
	}
	if req.Kind == providerKindAI {
		item.Priority = 100
	}
	if !req.IsActive {
		item.Status = "disabled"
	}
	if err := s.providerAccessor.Create(context.Background(), item); err != nil {
		return model.Provider{}, err
	}
	return *item, nil
}

func (s *TranslationService) UpdateProvider(_ context.Context, providerID string, req translationdto.UpdateProviderRequest) (model.Provider, error) {
	item, err := s.GetProvider(context.Background(), providerID)
	if err != nil {
		return model.Provider{}, err
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
	if err := s.providerAccessor.Update(context.Background(), &item); err != nil {
		return model.Provider{}, err
	}
	return item, nil
}

func (s *TranslationService) SetProviderStatus(_ context.Context, providerID string, active bool) (model.Provider, error) {
	item, err := s.GetProvider(context.Background(), providerID)
	if err != nil {
		return model.Provider{}, err
	}
	item.Status = "disabled"
	if active {
		item.Status = "active"
	}
	if err := s.providerAccessor.Update(context.Background(), &item); err != nil {
		return model.Provider{}, err
	}
	return item, nil
}

func (s *TranslationService) ListProviders(_ context.Context) []model.Provider {
	items, err := s.providerAccessor.List(context.Background())
	if err != nil {
		return []model.Provider{}
	}
	result := make([]model.Provider, 0, len(items))
	for _, item := range items {
		result = append(result, *item)
	}
	return result
}

func (s *TranslationService) GetProvider(_ context.Context, providerID string) (model.Provider, error) {
	item, err := s.providerAccessor.First(context.Background(), dataaccess.Eq("provider_key", providerID))
	if err != nil {
		return model.Provider{}, apperr.NotFound("translation provider %s not found", providerID)
	}
	return *item, nil
}

func cacheKey(item translationdto.TranslateItem) string {
	return "tr:v1:" + item.TargetLang + ":" + textHash(item.Text)
}

func textHash(text string) string {
	hash := sha256.Sum256([]byte(normalizeText(text)))
	return hex.EncodeToString(hash[:])
}

func normalizeText(text string) string {
	value := strings.TrimSpace(text)
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.Join(strings.Fields(value), " ")
}

func toCacheModel(item *model.Cache, result *translationdto.TranslateResult, provider *Provider) *model.Cache {
	result.ProviderID = provider.ID
	result.ProviderKind = provider.Kind
	result.ModelID = provider.Model
	result.Priority = provider.Priority
	result.QualityTier = providerKindMT
	if provider.Kind == providerKindAI {
		result.QualityTier = providerKindAI
	}
	result.ResultKey = resultKey(result.TargetLang, result.TranslatedText)
	item.CacheKey = cacheKey(translationdto.TranslateItem{
		Text:       result.Text,
		TargetLang: result.TargetLang,
	})
	item.TextHash = textHash(result.Text)
	item.SourceLang = result.SourceLang
	item.TargetLang = result.TargetLang
	item.Result = result.TranslatedText
	item.ProviderKind = provider.Kind
	item.ProviderID = provider.ID
	item.ModelID = provider.Model
	item.Priority = provider.Priority
	return item
}

func resultKey(targetLang, translatedText string) string {
	hash := sha256.Sum256([]byte(targetLang + "\n" + strings.TrimSpace(translatedText)))
	return hex.EncodeToString(hash[:])
}

func cachedToTranslateResult(item *model.Cache, originalText string) *translationdto.TranslateResult {
	qualityTier := providerKindMT
	if item.ProviderKind == providerKindAI {
		qualityTier = providerKindAI
	}
	return &translationdto.TranslateResult{
		Text:           originalText,
		TargetLang:     item.TargetLang,
		SourceLang:     item.SourceLang,
		TranslatedText: item.Result,
		Cached:         true,
		QualityTier:    qualityTier,
		ProviderID:     item.ProviderID,
		ProviderKind:   item.ProviderKind,
		ModelID:        item.ModelID,
		Priority:       item.Priority,
		ResultKey:      resultKey(item.TargetLang, item.Result),
	}
}

func (s *TranslationService) Translate(ctx context.Context, items []translationdto.TranslateItem) ([]translationdto.TranslateResult, error) {
	if len(items) == 0 {
		return []translationdto.TranslateResult{}, apperr.BadRequest("items is required")
	}
	results := make([]translationdto.TranslateResult, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.Text) == "" {
			return nil, apperr.BadRequest("items[].text is required")
		}
		if item.TargetLang == "" {
			item.TargetLang = "zh-CN"
		}
		result, err := s.translateItem(ctx, item)
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
	}
	return results, nil
}

func (s *TranslationService) translateItem(ctx context.Context, item translationdto.TranslateItem) (*translationdto.TranslateResult, error) {
	key := cacheKey(translationdto.TranslateItem{Text: item.Text, TargetLang: item.TargetLang})
	result, err, _ := s.sf.Do(key, func() (any, error) {
		providers, err := s.providers(ctx)
		if err != nil {
			return nil, err
		}
		if item.ProviderID != "" && !strings.EqualFold(item.ProviderID, autoProviderID) {
			for _, provider := range providers {
				if provider.ID == item.ProviderID {
					return s.translateWithProvider(ctx, provider, item, nil)
				}
			}
			return nil, apperr.NotFound("translation provider %s not found", item.ProviderID)
		}
		var cached *model.Cache
		if cachedValue, ok, _ := s.getCached(ctx, key); ok {
			cached = cachedValue
		}
		for _, provider := range providers {
			if cached != nil && cached.Priority >= provider.Priority {
				break
			}
			value, err := s.translateWithProvider(ctx, provider, item, cached)
			if err != nil {
				continue
			}
			if cacheErr := s.saveCache(ctx, value, provider); cacheErr != nil {
				return nil, cacheErr
			}
			return value, nil
		}
		if cached != nil {
			return cachedToTranslateResult(cached, item.Text), nil
		}
		return nil, apperr.New(502, "TRANSLATION_ERROR", "all translation providers failed")
	})
	if err != nil {
		return nil, err
	}
	translation, ok := result.(*translationdto.TranslateResult)
	if !ok {
		return nil, apperr.New(500, "INTERNAL", "translation result type mismatch")
	}
	return translation, nil
}

func (s *TranslationService) translateWithProvider(ctx context.Context, provider *Provider, item translationdto.TranslateItem, cached *model.Cache) (*translationdto.TranslateResult, error) {
	text, err := provider.Translate(ctx, item)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		return nil, apperr.New(502, "TRANSLATION_ERROR", "translation provider returned empty result")
	}
	result := &translationdto.TranslateResult{
		Text:           item.Text,
		TargetLang:     item.TargetLang,
		SourceLang:     item.SourceLang,
		TranslatedText: text,
		Cached:         cached != nil,
	}
	if cached == nil {
		result.Cached = false
	}
	result.ResultKey = resultKey(result.TargetLang, result.TranslatedText)
	result.TextHash = textHash(item.Text)
	return result, nil
}

func (s *TranslationService) providers(ctx context.Context) ([]*Provider, error) {
	result := make([]*Provider, 0)
	if s.aiService != nil && s.aiService.HasActiveProvider(ctx) {
		result = append(result, &Provider{
			ID:       "ai",
			Kind:     providerKindAI,
			Priority: 100,
			Translate: func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
				output, err := s.aiService.Generate(ctx, aipkg.GenerateRequest{
					System: "You are a high quality translation engine.",
					Prompt: "Translate to " + item.TargetLang + ": " + item.Text,
				})
				if err != nil {
					return "", err
				}
				return output.Text, nil
			},
		})
	}
	items, err := s.providerAccessor.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, provider := range items {
		if provider.Status != "active" {
			continue
		}
		if provider.Priority <= 0 {
			provider.Priority = 50
		}
		machineTranslateFn := s.machineTranslate(provider)
		if machineTranslateFn == nil {
			continue
		}
		result = append(result, &Provider{
			ID:        provider.ProviderKey,
			Kind:      providerKindMT,
			Endpoint:  provider.Endpoint,
			APIKeyEnv: provider.APIKeyEnv,
			Model:     provider.Model,
			Priority:  provider.Priority,
			Translate: machineTranslateFn,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority == result[j].Priority {
			return result[i].ID < result[j].ID
		}
		return result[i].Priority > result[j].Priority
	})
	return result, nil
}

func (s *TranslationService) machineTranslate(provider *model.Provider) func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
	switch provider.Kind {
	case "deeplx":
		return s.deeplxTranslate(provider)
	case "google":
		return s.googleTranslate(provider)
	case "deepl":
		return s.deeplWebTranslate(provider)
	case "mymemory":
		return s.myMemoryTranslate(provider)
	case "tencent":
		return s.tencentFreeTranslate(provider)
	case "volcengine":
		return s.volcengineFreeTranslate(provider)
	case "caiyun":
		return s.caiyunTranslate(provider)
	case "cnki":
		return s.cnkiTranslate(provider)
	case "papago":
		return s.papagoTranslate(provider)
	case "transmart":
		return s.transmartTranslate(provider)
	case "libretranslate":
		return func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
			options := []commontranslate.Option{}
			if provider.APIKeyEnv != "" {
				if value := os.Getenv(provider.APIKeyEnv); value != "" {
					options = append(options, commontranslate.WithAPIKey(value))
				}
			}
			client := commontranslate.NewClient(provider.Endpoint, options...)
			return client.Translate(ctx, item.Text, item.SourceLang, item.TargetLang)
		}
	default:
		return nil
	}
}

func (s *TranslationService) deeplxTranslate(provider *model.Provider) func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	return func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
		payload, err := json.Marshal(deeplxRequest{
			Text:       item.Text,
			SourceLang: item.SourceLang,
			TargetLang: item.TargetLang,
		})
		if err != nil {
			return "", err
		}
		httpReq, err := http.NewRequestWithContext(ctx, "POST", provider.Endpoint, bytes.NewReader(payload))
		if err != nil {
			return "", err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpResp, err := client.Do(httpReq)
		if err != nil {
			return "", err
		}
		defer httpResp.Body.Close()
		if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
			return "", fmt.Errorf("deeplx provider %s returned status %d", provider.ProviderKey, httpResp.StatusCode)
		}
		var response deeplxResponse
		if err := json.NewDecoder(httpResp.Body).Decode(&response); err != nil {
			return "", err
		}
		if response.Code != 0 || strings.TrimSpace(response.Data) == "" {
			return "", errors.New(response.Message)
		}
		return response.Data, nil
	}
}

func (s *TranslationService) getCached(ctx context.Context, key string) (*model.Cache, bool, error) {
	item, err := s.cacheAccessor.First(ctx, dataaccess.Eq("cache_key", key))
	if errors.Is(err, dataaccess.ErrDataNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return item, true, nil
}

func (s *TranslationService) saveCache(ctx context.Context, result *translationdto.TranslateResult, provider *Provider) error {
	item := &model.Cache{}
	toCacheModel(item, result, provider)
	cached, ok, err := s.getCached(ctx, item.CacheKey)
	if err != nil {
		return err
	}
	if ok && cached.Priority >= item.Priority {
		return nil
	}
	if ok {
		cached.Result = item.Result
		cached.SourceLang = item.SourceLang
		cached.ProviderKind = item.ProviderKind
		cached.ProviderID = item.ProviderID
		cached.ModelID = item.ModelID
		cached.Priority = item.Priority
		return s.cacheAccessor.Update(ctx, cached)
	}
	return s.cacheAccessor.Create(ctx, item)
}
