package api

import (
	nethttp "net/http"

	translationdto "cs-im-server/internal/module/translation/dto"
	"cs-im-server/internal/module/translation/internal/service"
	"cs-im-server/pkg/apperr"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/sse"
	"strconv"
	"time"
	commonmodule "vane-common/module"
)

type ProviderAPI struct {
	Service *service.TranslationService
}

func NewProviderAPI(translationService *service.TranslationService) *ProviderAPI {
	return &ProviderAPI{Service: translationService}
}

func (a *ProviderAPI) AdminRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("")
	routes.Get("/providers", a.ListProviders)
	routes.Post("/providers", a.CreateProvider)
	routes.Patch("/providers/:id", a.UpdateProvider)
	routes.Post("/providers/:id/enable", a.EnableProvider)
	routes.Post("/providers/:id/disable", a.DisableProvider)
	return routes
}

func (a *ProviderAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("")
	routes.Post("", a.Translate)
	return routes
}

func (a *ProviderAPI) Translate(ctx httpapi.Context) {
	var req translationdto.TranslateRequest
	if err := ctx.BindJSON(&req); err != nil {
		writeError(ctx, err)
		return
	}
	if ctx.Query("stream") == "true" {
		req.Stream = true
	}
	if req.Stream {
		a.streamTranslate(ctx, req.Items)
		return
	}
	results, err := a.Service.Translate(ctx.Context(), req.Items)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", results)
}

func (a *ProviderAPI) streamTranslate(ctx httpapi.Context, items []translationdto.TranslateItem) {
	sse.WriteHeaders(ctx)
	for i, item := range items {
		results, err := a.Service.Translate(ctx.Context(), []translationdto.TranslateItem{item})
		if err != nil {
			_ = sse.WriteEvent(ctx, sse.Event{Type: "translation.error", ID: strconv.Itoa(i), Data: err.Error(), At: time.Now()})
			continue
		}
		_ = sse.WriteEvent(ctx, sse.Event{Type: "translation.result", ID: strconv.Itoa(i), Data: results[0], At: time.Now()})
	}
}

func (a *ProviderAPI) ListProviders(ctx httpapi.Context) {
	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", a.Service.ListProviders(ctx.Context()))
}

func (a *ProviderAPI) CreateProvider(ctx httpapi.Context) {
	var req translationdto.CreateProviderRequest
	if err := ctx.BindJSON(&req); err != nil {
		writeError(ctx, err)
		return
	}
	provider, err := a.Service.CreateProvider(ctx.Context(), req)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", provider)
}

func (a *ProviderAPI) UpdateProvider(ctx httpapi.Context) {
	var req translationdto.UpdateProviderRequest
	if err := ctx.BindJSON(&req); err != nil {
		writeError(ctx, err)
		return
	}
	provider, err := a.Service.UpdateProvider(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", provider)
}

func (a *ProviderAPI) EnableProvider(ctx httpapi.Context) {
	provider, err := a.Service.SetProviderStatus(ctx.Context(), ctx.Param("id"), true)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", provider)
}

func (a *ProviderAPI) DisableProvider(ctx httpapi.Context) {
	provider, err := a.Service.SetProviderStatus(ctx.Context(), ctx.Param("id"), false)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", provider)
}

func writeError(ctx httpapi.Context, err error) {
	if serviceErr, ok := apperr.From(err); ok {
		httpapi.WriteError(ctx, serviceErr.Status, serviceErr.Code, serviceErr.Message)
		return
	}
	httpapi.WriteError(ctx, nethttp.StatusInternalServerError, "INTERNAL", err.Error())
}
