package api

import (
	nethttp "net/http"

	aidto "cs-im-server/internal/module/ai/dto"
	"cs-im-server/internal/module/ai/internal/service"
	"cs-im-server/pkg/apperr"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type ProviderAPI struct {
	Service *service.ProviderService
}

func NewProviderAPI(providerService *service.ProviderService) *ProviderAPI {
	return &ProviderAPI{Service: providerService}
}

func (a *ProviderAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/ai")
	routes.Get("/providers", a.List)
	routes.Post("/providers", a.Create)
	routes.Patch("/providers/:id", a.Update)
	routes.Post("/providers/:id/enable", a.Enable)
	routes.Post("/providers/:id/disable", a.Disable)
	routes.Post("/providers/:id/test", a.Test)
	routes.Post("/generations", a.Generate)
	return routes
}

func (a *ProviderAPI) List(ctx httpapi.Context) {
	providers := a.Service.ListProviders(ctx.Context())
	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", providers)
}

func (a *ProviderAPI) Create(ctx httpapi.Context) {
	var req aidto.CreateProviderRequest
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

func (a *ProviderAPI) Update(ctx httpapi.Context) {
	var req aidto.UpdateProviderRequest
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

func (a *ProviderAPI) Enable(ctx httpapi.Context) {
	provider, err := a.Service.SetProviderStatus(ctx.Context(), ctx.Param("id"), true)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", provider)
}

func (a *ProviderAPI) Disable(ctx httpapi.Context) {
	provider, err := a.Service.SetProviderStatus(ctx.Context(), ctx.Param("id"), false)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", provider)
}

func (a *ProviderAPI) Test(ctx httpapi.Context) {
	result, err := a.Service.Generate(ctx.Context(), aidto.GenerateRequest{
		System: "You are a health check endpoint. Reply with pong.",
		Prompt: "test",
	})
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", result)
}

func (a *ProviderAPI) Generate(ctx httpapi.Context) {
	var req aidto.GenerateRequest
	if err := ctx.BindJSON(&req); err != nil {
		writeError(ctx, err)
		return
	}
	result, err := a.Service.Generate(ctx.Context(), req)
	if err != nil {
		writeError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, nethttp.StatusOK, "OK", result)
}

func writeError(ctx httpapi.Context, err error) {
	if serviceErr, ok := apperr.From(err); ok {
		httpapi.WriteError(ctx, serviceErr.Status, serviceErr.Code, serviceErr.Message)
		return
	}
	httpapi.WriteError(ctx, nethttp.StatusInternalServerError, "INTERNAL", err.Error())
}
