package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/internal/module/llm/internal/service"
	llmpkg "cs-im-server/internal/module/llm/pkg"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

const (
	chatPath     = "/chat/completions"
	messagesPath = "/messages"
	modelsPath   = "/models"
)

type API struct{ service *service.Service }

func New(llmService *service.Service) *API { return &API{service: llmService} }

func (a *API) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("")
	routes.Post(chatPath, a.ChatCompletions)
	routes.Post(messagesPath, a.Messages)
	routes.Get(modelsPath, a.Models)
	return routes
}

func (a *API) ChatCompletions(ctx httpapi.Context) { a.proxy(ctx, false) }
func (a *API) Messages(ctx httpapi.Context)        { a.proxy(ctx, true) }

func (a *API) proxy(ctx httpapi.Context, anthropic bool) {
	principal, ok := iam.PrincipalFromContext(ctx.Context())
	if !ok || !principal.HasKind(iam.PrincipalKindHuman) {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "valid staff session required")
		return
	}
	if !principal.HasScope(iam.ScopeGatewayAdmin) {
		httpapi.WriteError(ctx, http.StatusForbidden, "FORBIDDEN", "llm access is not allowed")
		return
	}
	requestLimit := a.service.MaxRequestBytes()
	if requestLimit <= 0 {
		requestLimit = 1 << 20
	}
	body, err := io.ReadAll(io.LimitReader(ctx.Request().Body, requestLimit+1))
	if err != nil {
		writeProtocolError(ctx, anthropic, http.StatusBadRequest, "invalid_request_error", "could not read request body")
		return
	}
	if int64(len(body)) > requestLimit {
		writeProtocolError(ctx, anthropic, http.StatusRequestEntityTooLarge, "invalid_request_error", "request body exceeds limit")
		return
	}
	selector, err := requestModel(body)
	if err != nil {
		writeProtocolError(ctx, anthropic, http.StatusBadRequest, "invalid_request_error", "model selector is required")
		return
	}
	sessionHeader := ctx.Header("X-OpenCode-Session")
	if sessionHeader == "" {
		sessionHeader = ctx.Header("X-Session-ID")
	}
	sessionID := llmpkg.SessionID(sessionHeader)
	_ = ctx.SetWriteDeadline(time.Now().Add(10 * time.Minute))
	var upstream *http.Response
	if anthropic {
		upstream, err = a.service.Messages(ctx.Context(), selector, body, sessionID)
	} else {
		upstream, err = a.service.Chat(ctx.Context(), selector, body, sessionID)
		if err == nil {
			if strings.Contains(strings.ToLower(upstream.Header.Get("Content-Type")), "text/event-stream") {
				upstream = a.service.WrapMiniMaxStream(selector, upstream)
			} else {
				upstream, err = a.service.WrapMiniMaxResponse(selector, upstream)
			}
		}
	}
	if err != nil {
		writeLLMError(ctx, err, anthropic)
		return
	}
	defer upstream.Body.Close()
	if upstream.StatusCode < 200 || upstream.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(upstream.Body, 4096))
		if readErr != nil {
			writeProtocolError(ctx, anthropic, http.StatusBadGateway, "api_error", "upstream model request failed")
			return
		}
		if upstream.Header.Get("Content-Type") != "" {
			ctx.SetHeader("Content-Type", upstream.Header.Get("Content-Type"))
		} else {
			ctx.SetHeader("Content-Type", "application/json; charset=utf-8")
		}
		ctx.Status(upstream.StatusCode)
		if anthropic {
			body = anthropicErrorBody(body)
		}
		_, _ = ctx.Write(body)
		return
	}
	for _, key := range []string{"Content-Type", "Cache-Control", "X-Request-Id", "Request-Id", "Anthropic-Version"} {
		if value := upstream.Header.Get(key); value != "" {
			ctx.SetHeader(key, value)
		}
	}
	ctx.Status(upstream.StatusCode)
	if isEventStream(upstream.Header.Get("Content-Type")) {
		buffer := make([]byte, 32*1024)
		for {
			n, readErr := upstream.Body.Read(buffer)
			if n > 0 {
				if _, writeErr := ctx.Write(buffer[:n]); writeErr != nil {
					return
				}
				_ = ctx.Flush()
			}
			if readErr != nil {
				if readErr != io.EOF {
					writeStreamError(ctx, anthropic)
					_ = ctx.Flush()
				}
				return
			}
		}
	}
	_, _ = io.Copy(ctx, upstream.Body)
}

func anthropicErrorBody(body []byte) []byte {
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return []byte("{\"type\":\"error\",\"error\":{\"type\":\"api_error\",\"message\":\"upstream model request failed\"}}")
	}
	if payload["type"] == "error" && payload["error"] != nil {
		return body
	}
	result, err := json.Marshal(map[string]any{"type": "error", "error": map[string]string{"type": "api_error", "message": "upstream model request failed"}})
	if err != nil {
		return []byte("{\"type\":\"error\",\"error\":{\"type\":\"api_error\",\"message\":\"upstream model request failed\"}}")
	}
	return result
}

func (a *API) Models(ctx httpapi.Context) {
	principal, ok := iam.PrincipalFromContext(ctx.Context())
	if !ok || !principal.HasKind(iam.PrincipalKindHuman) {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "valid staff session required")
		return
	}
	if !principal.HasScope(iam.ScopeGatewayAdmin) {
		httpapi.WriteError(ctx, http.StatusForbidden, "FORBIDDEN", "llm access is not allowed")
		return
	}
	models, err := a.service.ListModels()
	if err != nil {
		writeLLMError(ctx, err, false)
		return
	}
	ctx.SetHeader("Content-Type", "application/json; charset=utf-8")
	ctx.Status(http.StatusOK)
	if len(models) == 0 {
		models = []service.ModelInfo{}
	}
	body, _ := json.Marshal(map[string]any{"object": "list", "data": models})
	_, _ = ctx.Write(body)
}

func requestModel(body []byte) (string, error) {
	var payload struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", err
	}
	selector := strings.TrimSpace(payload.Model)
	if selector == "" {
		return "", service.ErrInvalidRequest
	}
	return selector, nil
}

func writeLLMError(ctx httpapi.Context, err error, anthropic ...bool) {
	status := http.StatusBadGateway
	code := "api_error"
	message := "upstream model request failed"
	switch {
	case errors.Is(err, service.ErrDisabled):
		status, code, message = http.StatusServiceUnavailable, "api_error", "LLM gateway is disabled"
	case errors.Is(err, service.ErrModelNotFound):
		status, code, message = http.StatusNotFound, "not_found_error", "model is not configured or disabled"
	case errors.Is(err, service.ErrInvalidRequest):
		status, code, message = http.StatusBadRequest, "invalid_request_error", "invalid model request"
	case errors.Is(err, service.ErrTooLarge):
		status, code, message = http.StatusRequestEntityTooLarge, "invalid_request_error", "request or response exceeded limit"
	}
	writeProtocolError(ctx, len(anthropic) > 0 && anthropic[0], status, code, message)
}

func writeProtocolError(ctx httpapi.Context, anthropic bool, status int, code, message string) {
	ctx.SetHeader("Content-Type", "application/json; charset=utf-8")
	ctx.Status(status)
	var body []byte
	if anthropic {
		body, _ = json.Marshal(map[string]any{"type": "error", "error": map[string]string{"type": code, "message": message}})
	} else {
		body, _ = json.Marshal(map[string]any{"error": map[string]string{"type": code, "message": message}})
	}
	_, _ = ctx.Write(body)
}

func writeStreamError(ctx httpapi.Context, anthropic bool) {
	if anthropic {
		_, _ = ctx.Write([]byte("event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"api_error\",\"message\":\"upstream model stream failed\"}}\n\n"))
		return
	}
	_, _ = ctx.Write([]byte("data: {\"error\":{\"type\":\"api_error\",\"message\":\"upstream model stream failed\"}}\n\n"))
}

func isEventStream(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/event-stream")
}
