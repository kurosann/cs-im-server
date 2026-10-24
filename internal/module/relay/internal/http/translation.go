package http

import (
	"net/http"

	relaydto "cs-im-server/internal/module/relay/dto"
	translationpkg "cs-im-server/internal/module/translation/pkg/translation"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type RelayTranslationAPI struct {
	Translations *translationpkg.TranslationService
}

func NewRelayTranslationAPI(translations *translationpkg.TranslationService) *RelayTranslationAPI {
	return &RelayTranslationAPI{Translations: translations}
}

func (a *RelayTranslationAPI) Translate(ctx httpapi.Context) {
	var req relaydto.RelayTranslateRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	items := make([]translationpkg.TranslateItem, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, translationpkg.TranslateItem(item))
	}
	result, err := a.Translations.Translate(ctx.Context(), items)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	results := make([]relaydto.RelayTranslateResult, 0, len(result))
	for _, item := range result {
		results = append(results, relaydto.RelayTranslateResult{
			Text:           item.Text,
			TargetLang:     item.TargetLang,
			SourceLang:     item.SourceLang,
			TranslatedText: item.TranslatedText,
			Cached:         item.Cached,
			QualityTier:    item.QualityTier,
		})
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", results)
}

func (a *RelayTranslationAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("")
	routes.Post("/translations", a.Translate)
	return routes
}
