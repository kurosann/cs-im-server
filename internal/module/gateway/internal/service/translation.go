package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	maxTranslationTextLength = 5000
	serverTranslationTimeout = 8 * time.Second
)

type TranslationService struct{}

func NewTranslationService() *TranslationService {
	return &TranslationService{}
}

func (s *TranslationService) Translate(
	ctx context.Context,
	text string,
	sourceLang string,
	targetLang string,
) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", errors.New("text is required")
	}
	if utf8.RuneCountInString(text) > maxTranslationTextLength {
		return "", errors.New("text exceeds maximum length")
	}
	sourceLang = strings.TrimSpace(sourceLang)
	targetLang = strings.TrimSpace(targetLang)
	if sourceLang == "" {
		sourceLang = "auto"
	}
	if targetLang == "" || targetLang == "auto" {
		targetLang = oppositeLanguage(text, sourceLang)
	}

	requestCtx, cancel := context.WithTimeout(ctx, serverTranslationTimeout)
	defer cancel()

	query := url.Values{
		"client": {"gtx"},
		"sl":     {sourceLang},
		"tl":     {targetLang},
		"dt":     {"t"},
		"dj":     {"1"},
		"ie":     {"UTF-8"},
		"q":      {text},
	}
	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodGet,
		"https://translate.googleapis.com/translate_a/single?"+query.Encode(),
		nil,
	)
	if err != nil {
		return "", err
	}
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")
	response, err := (&http.Client{Timeout: serverTranslationTimeout}).Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return "", errors.New("translation provider returned a non-success status")
	}
	var payload map[string]any
	if err := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(&payload); err != nil {
		return "", err
	}
	sentences, _ := payload["sentences"].([]any)
	var translated strings.Builder
	for _, item := range sentences {
		sentence, _ := item.(map[string]any)
		part, _ := sentence["trans"].(string)
		translated.WriteString(part)
	}
	result := strings.TrimSpace(translated.String())
	if result == "" {
		return "", errors.New("translation returned empty text")
	}
	return result, nil
}

func oppositeLanguage(text string, sourceLang string) string {
	if sourceLang != "" && sourceLang != "auto" {
		normalizedSource := strings.ToLower(sourceLang)
		if strings.HasPrefix(normalizedSource, "zh") {
			return "en"
		}
		return "zh-CN"
	}
	for _, char := range text {
		if unicode.In(char, unicode.Han) {
			return "en"
		}
	}
	return "zh-CN"
}
