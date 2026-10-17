package http

import (
	"net/http"

	"cs-im-server/internal/module/relay/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type WebhookAPI struct {
	Service *service.RelayService
}

func NewWebhookAPI(relayService *service.RelayService) *WebhookAPI {
	return &WebhookAPI{Service: relayService}
}

func (a *WebhookAPI) Create(ctx httpapi.Context) {
	var req service.CreateWebhookRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	webhook, err := a.Service.CreateWebhook(ctx.Context(), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", webhook)
}

func (a *WebhookAPI) List(ctx httpapi.Context) {
	webhooks := a.Service.ListWebhooks(ctx.Context())
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", webhooks)
}

func (a *WebhookAPI) Get(ctx httpapi.Context) {
	webhook, err := a.Service.GetWebhook(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", webhook)
}

func (a *WebhookAPI) Update(ctx httpapi.Context) {
	var req service.UpdateWebhookRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	webhook, err := a.Service.UpdateWebhook(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", webhook)
}

func (a *WebhookAPI) Test(ctx httpapi.Context) {
	webhook, err := a.Service.TestWebhook(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", webhook)
}

func (a *WebhookAPI) Enable(ctx httpapi.Context) {
	webhook, err := a.Service.SetWebhookStatus(ctx.Context(), ctx.Param("id"), true)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", webhook)
}

func (a *WebhookAPI) Disable(ctx httpapi.Context) {
	webhook, err := a.Service.SetWebhookStatus(ctx.Context(), ctx.Param("id"), false)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", webhook)
}

func (a *WebhookAPI) Delete(ctx httpapi.Context) {
	if err := a.Service.DeleteWebhook(ctx.Context(), ctx.Param("id")); err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", nil)
}

func (a *WebhookAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/webhooks")
	routes.Post("", a.Create)
	routes.Get("", a.List)
	routes.Get("/:id", a.Get)
	routes.Patch("/:id", a.Update)
	routes.Post("/:id/test", a.Test)
	routes.Post("/:id/enable", a.Enable)
	routes.Post("/:id/disable", a.Disable)
	routes.Delete("/:id", a.Delete)
	return routes
}
