package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"cs-im-server/internal/module/gateway/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type TranslationAPI struct {
	Service *service.TranslationService
}

func NewTranslationAPI(translationService *service.TranslationService) *TranslationAPI {
	return &TranslationAPI{Service: translationService}
}

func (a *TranslationAPI) Translate(ctx httpapi.Context) {
	var request gatewaydto.TranslateRequest
	if err := ctx.BindJSON(&request); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}
	if strings.TrimSpace(request.Text) == "" {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "text is required")
		return
	}
	if utf8.RuneCountInString(request.Text) > 5000 {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "text exceeds maximum length")
		return
	}
	translated, err := a.Service.Translate(ctx.Context(), request.Text, request.SourceLang, request.TargetLang)
	if err != nil {
		httpapi.WriteError(ctx, http.StatusBadGateway, "TRANSLATION_FAILED", "server translation is unavailable")
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", gatewaydto.TranslateResponse{
		Text:     translated,
		Provider: "Google 翻译（服务器）",
	})
}

func (a *TranslationAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/translate")
	routes.Post("", a.Translate)
	return routes
}
