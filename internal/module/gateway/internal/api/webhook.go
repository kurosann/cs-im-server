package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type WebhookAPI struct{}

func (a *WebhookAPI) Create(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "webhook create entrypoint reserved")
}

func (a *WebhookAPI) List(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "webhook list entrypoint reserved")
}

func (a *WebhookAPI) Get(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "webhook get entrypoint reserved")
}

func (a *WebhookAPI) Update(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "webhook update entrypoint reserved")
}

func (a *WebhookAPI) Test(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "webhook test entrypoint reserved")
}

func (a *WebhookAPI) Enable(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "webhook enable entrypoint reserved")
}

func (a *WebhookAPI) Disable(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "webhook disable entrypoint reserved")
}

func (a *WebhookAPI) Delete(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "webhook delete entrypoint reserved")
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
