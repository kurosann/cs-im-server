package service

import (
	"bytes"
	"context"
	"crypto/aes"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	whatlanggo "github.com/abadojack/whatlanggo"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	translationdto "cs-im-server/internal/module/translation/dto"
	"cs-im-server/internal/module/translation/internal/model"
)

const (
	googleTranslateEndpoint     = "https://translate.googleapis.com/translate_a/single"
	deeplWebEndpoint            = "https://www2.deepl.com/jsonrpc"
	mymemoryTranslateEndpoint   = "https://api.mymemory.translated.net/get"
	tencentTranslateEndpoint    = "https://wxapp.translator.qq.com/api/translate"
	volcengineTranslateEndpoint = "https://translate.volcengine.com/crx/translate/v1"
	caiyunTranslateEndpoint     = "https://interpreter.cyapi.cn/v1/translator"
	cnkiTokenEndpoint           = "https://dict.cnki.net/fyzs-front-api/getToken"
	cnkiTranslateEndpoint       = "https://dict.cnki.net/fyzs-front-api/translate/literalTranslation"
	papagoDetectEndpoint        = "https://api.papago-chrome.com/v2/translate/detect"
	papagoTranslateEndpoint     = "https://api.papago-chrome.com/v2/translate/openapi"
	transmartTranslateEndpoint  = "https://yi.qq.com/api/imt"
)

type httpClient struct {
	client *http.Client
}

type translateFunc func(ctx context.Context, item translationdto.TranslateItem) (string, error)

func (h httpClient) doJSON(ctx context.Context, method, endpoint string, headers map[string]string, body any, result any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	} else if method == http.MethodPost {
		reader = bytes.NewReader([]byte{})
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	for key, value := range headers {
		httpReq.Header.Set(key, value)
	}
	if httpReq.Header.Get("User-Agent") == "" {
		httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36")
	}
	httpResp, err := h.client.Do(httpReq)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	responseBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("http %d: %s", httpResp.StatusCode, truncate(string(responseBody), 240))
	}
	return json.Unmarshal(responseBody, result)
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}

func endpointOrDefault(provider *model.Provider, fallback string) string {
	value := strings.TrimSpace(provider.Endpoint)
	if value == "" {
		return fallback
	}
	return value
}

func inferSourceLang(text string) string {
	detected := whatlanggo.DetectLang(text).Iso6391()
	if detected == "" {
		return "en"
	}
	return detected
}

func sourceLang(item translationdto.TranslateItem) string {
	value := strings.TrimSpace(item.SourceLang)
	if value == "" || strings.EqualFold(value, "auto") {
		return inferSourceLang(item.Text)
	}
	return value
}

func normalizeLang(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch {
	case value == "", value == "auto":
		return "auto"
	case value == "zh-cn", value == "zh-hans":
		return "zh"
	case value == "zh-tw", value == "zh-hant":
		return "zh-TW"
	case strings.HasPrefix(value, "en-"):
		return "en"
	case strings.HasPrefix(value, "pt-"):
		return "pt"
	}
	return value
}

func (s *TranslationService) googleTranslate(provider *model.Provider) translateFunc {
	client := httpClient{client: &http.Client{Timeout: 12 * time.Second}}
	return func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
		requestURL, err := url.Parse(endpointOrDefault(provider, googleTranslateEndpoint))
		if err != nil {
			return "", err
		}
		query := requestURL.Query()
		query.Set("client", "gtx")
		query.Set("sl", sourceLang(item))
		query.Set("tl", normalizeLang(item.TargetLang))
		query.Set("dt", "t")
		query.Set("dj", "1")
		query.Set("q", item.Text)
		requestURL.RawQuery = query.Encode()
		var response struct {
			Sentences []struct {
				Trans string `json:"trans"`
			} `json:"sentences"`
			SourceLang string `json:"src"`
		}
		err = client.doJSON(ctx, http.MethodGet, requestURL.String(), map[string]string{
			"accept-language": "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7",
		}, nil, &response)
		if err != nil {
			return "", err
		}
		var builder strings.Builder
		for _, sentence := range response.Sentences {
			builder.WriteString(sentence.Trans)
		}
		return strings.TrimSpace(builder.String()), nil
	}
}

func getDeepLLang(value, fallback string) string {
	normalized := normalizeLang(value)
	if normalized == "auto" {
		if fallback != "" {
			return fallback
		}
		return "auto"
	}
	switch normalized {
	case "zh":
		return "ZH-HANS"
	case "zh-TW":
		return "ZH-HANT"
	case "en":
		return "EN"
	case "pt":
		return "PT"
	}
	return strings.ToUpper(normalized)
}

func (s *TranslationService) deeplWebTranslate(provider *model.Provider) translateFunc {
	client := httpClient{client: &http.Client{Timeout: 12 * time.Second}}
	return func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
		sourceLang := getDeepLLang(sourceLang(item), "auto")
		targetLang := getDeepLLang(item.TargetLang, "ZH-HANS")
		effectiveTargetLang := targetLang
		if targetLang == "ZH-HANS" || targetLang == "ZH-HANT" {
			effectiveTargetLang = "ZH"
		}
		iCount := strings.Count(item.Text, "i")
		timestamp := time.Now().UnixMilli()
		if iCount > 0 {
			timestamp = timestamp - timestamp%(int64(iCount)+1) + int64(iCount) + 1
		}
		id, err := cryptorand.Int(cryptorand.Reader, big.NewInt(99999))
		if err != nil {
			return "", err
		}
		jsonRPCID := (id.Int64() + 100000) * 1000
		params := map[string]any{
			"texts":     []map[string]any{{"text": item.Text, "requestAlternatives": 3}},
			"splitting": "newlines",
			"lang": map[string]any{
				"source_lang_user_selected": sourceLang,
				"target_lang":               effectiveTargetLang,
			},
			"timestamp": timestamp,
		}
		if targetLang == "ZH-HANS" || targetLang == "ZH-HANT" {
			params["commonJobParams"] = map[string]any{"regionalVariant": targetLang}
		}
		payload := map[string]any{
			"jsonrpc": "2.0",
			"method":  "LMT_handle_texts",
			"id":      jsonRPCID,
			"params":  params,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return "", err
		}
		if (jsonRPCID+5)%29 == 0 || (jsonRPCID+3)%13 == 0 {
			body = bytes.ReplaceAll(body, []byte(`"method":"`), []byte(`"method" : "`))
		} else {
			body = bytes.ReplaceAll(body, []byte(`"method":"`), []byte(`"method": "`))
		}
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointOrDefault(provider, deeplWebEndpoint), bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		httpReq.Header.Set("accept", "application/json")
		httpReq.Header.Set("content-type", "application/json")
		httpResp, err := client.client.Do(httpReq)
		if err != nil {
			return "", err
		}
		defer httpResp.Body.Close()
		responseBody, err := io.ReadAll(httpResp.Body)
		if err != nil {
			return "", err
		}
		if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
			return "", fmt.Errorf("http %d: %s", httpResp.StatusCode, truncate(string(responseBody), 240))
		}
		var response struct {
			Result struct {
				Texts []struct {
					Text string `json:"text"`
				} `json:"texts"`
			} `json:"result"`
		}
		if err := json.Unmarshal(responseBody, &response); err != nil {
			return "", err
		}
		if len(response.Result.Texts) == 0 {
			return "", errors.New("empty-translation")
		}
		return strings.TrimSpace(response.Result.Texts[0].Text), nil
	}
}

func (s *TranslationService) myMemoryTranslate(provider *model.Provider) translateFunc {
	client := httpClient{client: &http.Client{Timeout: 12 * time.Second}}
	return func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
		requestURL, err := url.Parse(endpointOrDefault(provider, mymemoryTranslateEndpoint))
		if err != nil {
			return "", err
		}
		query := requestURL.Query()
		query.Set("q", item.Text)
		query.Set("langpair", sourceLang(item)+"|"+normalizeLang(item.TargetLang))
		requestURL.RawQuery = query.Encode()
		var response struct {
			ResponseData struct {
				TranslatedText string `json:"translatedText"`
			} `json:"responseData"`
			ResponseDetails string `json:"responseDetails"`
		}
		if err := client.doJSON(ctx, http.MethodGet, requestURL.String(), map[string]string{"accept": "application/json"}, nil, &response); err != nil {
			return "", err
		}
		return strings.TrimSpace(response.ResponseData.TranslatedText), nil
	}
}

func (s *TranslationService) tencentFreeTranslate(provider *model.Provider) translateFunc {
	client := httpClient{client: &http.Client{Timeout: 12 * time.Second}}
	return func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
		from := sourceLang(item)
		target := normalizeLang(item.TargetLang)
		if target == "auto" {
			target = "zh"
		}
		requestURL, err := url.Parse(endpointOrDefault(provider, tencentTranslateEndpoint))
		if err != nil {
			return "", err
		}
		query := requestURL.Query()
		query.Set("source", "auto")
		query.Set("target", "auto")
		query.Set("sourceText", item.Text)
		query.Set("platform", "WeChat_APP")
		query.Set("candidateLangs", from+"|"+target)
		query.Set("guid", "oqdgX0SIwhvM0TmqzTHghWBvfk22")
		requestURL.RawQuery = query.Encode()
		var response struct {
			TargetText string `json:"targetText"`
		}
		if err := client.doJSON(ctx, http.MethodGet, requestURL.String(), map[string]string{
			"accept":  "application/json",
			"referer": "https://servicewechat.com/wxb1070eabc6f9107e/117/page-frame.html",
		}, nil, &response); err != nil {
			return "", err
		}
		return strings.TrimSpace(response.TargetText), nil
	}
}

func (s *TranslationService) volcengineFreeTranslate(provider *model.Provider) translateFunc {
	client := httpClient{client: &http.Client{Timeout: 12 * time.Second}}
	return func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
		from := sourceLang(item)
		target := normalizeLang(item.TargetLang)
		if target == "auto" {
			target = "en"
		}
		var response struct {
			Translation string `json:"translation"`
		}
		if err := client.doJSON(ctx, http.MethodPost, endpointOrDefault(provider, volcengineTranslateEndpoint), map[string]string{
			"content-type": "application/json",
			"origin":       "chrome-extension://klgfhbdadaspgppeadghjjemk",
		}, map[string]string{
			"source_language": from,
			"target_language": target,
			"text":            item.Text,
		}, &response); err != nil {
			return "", err
		}
		return strings.TrimSpace(response.Translation), nil
	}
}

func (s *TranslationService) caiyunTranslate(provider *model.Provider) translateFunc {
	client := httpClient{client: &http.Client{Timeout: 12 * time.Second}}
	return func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
		from := sourceLang(item)
		target := normalizeLang(item.TargetLang)
		if target == "auto" {
			target = "en"
		}
		transType := from + "2" + target
		if from == "auto" {
			transType = "auto2" + target
		}
		var response struct {
			Target string `json:"target"`
		}
		authorization := ""
		if provider.APIKeyEnv != "" {
			authorization = "token " + os.Getenv(provider.APIKeyEnv)
		}
		if err := client.doJSON(ctx, http.MethodPost, endpointOrDefault(provider, caiyunTranslateEndpoint), map[string]string{
			"content-type":    "application/json",
			"x-authorization": authorization,
			"user-agent":      "caiyunInterpreter/5 CFNetwork/1404.0.5 Darwin/22.3.0",
		}, map[string]any{
			"source":     item.Text,
			"detect":     from == "auto",
			"os_type":    "ios",
			"device_id":  "F1F902F7-1780-4C88-848D-71F35D88A602",
			"trans_type": transType,
			"media":      "text",
			"request_id": time.Now().UnixMilli(),
			"user_id":    "",
			"dict":       true,
		}, &response); err != nil {
			return "", err
		}
		return strings.TrimSpace(response.Target), nil
	}
}

func encryptCNKIText(text string) (string, error) {
	block, err := aes.NewCipher([]byte("95bae0e3871c9834"))
	if err != nil {
		return "", err
	}
	padding := aes.BlockSize - len(text)%aes.BlockSize
	padded := append([]byte(text), bytes.Repeat([]byte{byte(padding)}, padding)...)
	encrypted := make([]byte, len(padded))
	for offset := 0; offset < len(padded); offset += block.BlockSize() {
		block.Encrypt(encrypted[offset:offset+block.BlockSize()], padded[offset:offset+block.BlockSize()])
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func (s *TranslationService) cnkiTranslate(provider *model.Provider) translateFunc {
	client := httpClient{client: &http.Client{Timeout: 12 * time.Second}}
	headers := map[string]string{
		"content-type": "application/json;charset=UTF-8",
		"origin":       "https://dict.cnki.net",
		"referer":      "https://dict.cnki.net/index",
	}
	return func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
		encryptedText, err := encryptCNKIText(item.Text)
		if err != nil {
			return "", err
		}
		var tokenResponse struct {
			Code    int    `json:"code"`
			Data    string `json:"data"`
			Message string `json:"message"`
		}
		if err := client.doJSON(ctx, http.MethodGet, cnkiTokenEndpoint, headers, nil, &tokenResponse); err != nil {
			return "", err
		}
		if tokenResponse.Data == "" {
			return "", errors.New("missing-cnki-token")
		}
		request := map[string]any{"words": encryptedText, "translateType": nil}
		headers["token"] = tokenResponse.Data
		var response struct {
			Code int `json:"code"`
			Data struct {
				Result string `json:"mResult"`
			} `json:"data"`
			Message string `json:"message"`
		}
		if err := client.doJSON(ctx, http.MethodPost, cnkiTranslateEndpoint, headers, request, &response); err != nil {
			return "", err
		}
		return strings.TrimSpace(response.Data.Result), nil
	}
}

func (s *TranslationService) papagoTranslate(provider *model.Provider) translateFunc {
	client := httpClient{client: &http.Client{Timeout: 12 * time.Second}}
	headers := map[string]string{
		"accept":       "application/json, text/javascript, */*; q=0.01",
		"content-type": "application/json; charset=UTF-8",
		"origin":       "chrome-extension://enddgifdbfoefnelepppgaabobdfbcpe",
	}
	return func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
		from := sourceLang(item)
		target := normalizeLang(item.TargetLang)
		if target == "auto" {
			if from == "ko" {
				target = "en"
			} else {
				target = "ko"
			}
		}
		var response struct {
			TranslatedText string `json:"translatedText"`
			Message        string `json:"message"`
		}
		if err := client.doJSON(ctx, http.MethodPost, endpointOrDefault(provider, papagoTranslateEndpoint), headers, map[string]string{
			"source": from,
			"target": target,
			"text":   item.Text,
		}, &response); err != nil {
			return "", err
		}
		return strings.TrimSpace(response.TranslatedText), nil
	}
}

func (s *TranslationService) transmartTranslate(provider *model.Provider) translateFunc {
	client := httpClient{client: &http.Client{Timeout: 12 * time.Second}}
	return func(ctx context.Context, item translationdto.TranslateItem) (string, error) {
		from := sourceLang(item)
		target := normalizeLang(item.TargetLang)
		if target == "auto" {
			target = "en"
		}
		requestID, err := cryptorand.Int(cryptorand.Reader, big.NewInt(999999))
		if err != nil {
			return "", err
		}
		var response struct {
			AutoTranslation []string `json:"auto_translation"`
			Message         string   `json:"message"`
		}
		if err := client.doJSON(ctx, http.MethodPost, endpointOrDefault(provider, transmartTranslateEndpoint), map[string]string{
			"content-type": "application/json",
			"referer":      "https://yi.qq.com/zh-CN/index",
		}, map[string]any{
			"header": map[string]any{
				"fn":         "auto_translation",
				"client_key": fmt.Sprintf("browser-chrome-142.0.0-Mac OS-%016x-%d", requestID.Int64(), time.Now().UnixMilli()),
			},
			"type":           "plain",
			"model_category": "normal",
			"source": map[string]any{
				"lang":      from,
				"text_list": []string{item.Text},
			},
			"target": map[string]any{
				"lang": target,
			},
		}, &response); err != nil {
			return "", err
		}
		if len(response.AutoTranslation) == 0 {
			return "", errors.New("empty-translation")
		}
		return strings.Join(response.AutoTranslation, "\n"), nil
	}
}
