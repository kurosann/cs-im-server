package service

import (
	"context"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	translationdto "cs-im-server/internal/module/translation/dto"
	"cs-im-server/internal/module/translation/internal/model"
)

func TestCacheKeyIsTextAndLanguageOnly(t *testing.T) {
	first := cacheKey(translationdto.TranslateItem{Text: "Hello world", TargetLang: "zh-CN"})
	second := cacheKey(translationdto.TranslateItem{Text: "Hello \tworld", TargetLang: "zh-CN"})
	other := cacheKey(translationdto.TranslateItem{Text: "Hello world", TargetLang: "en"})

	if first != second {
		t.Fatalf("cacheKey(normalized text) = %s, want %s", second, first)
	}
	if first == other {
		t.Fatalf("cacheKey(other language) should differ")
	}
}

func TestToCacheModelKeepsProviderAndPriority(t *testing.T) {
	cache := &model.Cache{}
	provider := &Provider{ID: "openai-gpt", Kind: providerKindAI, Model: "gpt", Priority: 100}
	cacheResult := toCacheModel(cache, &translationdto.TranslateResult{
		Text:           "hello",
		TargetLang:     "zh-CN",
		SourceLang:     "auto",
		TranslatedText: "你好",
	}, provider)

	if cache.CacheKey != cacheKey(translationdto.TranslateItem{Text: "hello", TargetLang: "zh-CN"}) {
		t.Fatalf("cache key = %s", cache.CacheKey)
	}
	if cache.Priority != 100 || cacheResult.ProviderKind != providerKindAI {
		t.Fatalf("cache priority/kind = %d/%s", cache.Priority, cacheResult.ProviderKind)
	}
}

func TestGoogleWebTranslateUsesConfiguredEndpoint(t *testing.T) {
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.URL.Query().Get("sl") != "ja" || r.URL.Query().Get("tl") != "zh" {
			nethttp.Error(w, "bad lang", nethttp.StatusBadRequest)
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"sentences":[{"trans":"你好"}],"src":"ja"}`))
	}))
	defer server.Close()

	service := NewTranslationService(nil)
	result, err := service.googleTranslate(&model.Provider{Endpoint: server.URL})(context.Background(), translationdto.TranslateItem{
		Text:       "こんにちは",
		TargetLang: "zh-CN",
	})
	if err != nil || result != "你好" {
		t.Fatalf("googleTranslate() = %q, %v", result, err)
	}
}

func TestVolcengineFreeTranslate(t *testing.T) {
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			nethttp.Error(w, err.Error(), nethttp.StatusBadRequest)
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"translation":"Hello"}`))
	}))
	defer server.Close()

	service := NewTranslationService(nil)
	result, err := service.volcengineFreeTranslate(&model.Provider{Endpoint: server.URL})(context.Background(), translationdto.TranslateItem{
		Text:       "你好",
		TargetLang: "en",
	})
	if err != nil || result != "Hello" {
		t.Fatalf("volcengineFreeTranslate() = %q, %v", result, err)
	}
}
