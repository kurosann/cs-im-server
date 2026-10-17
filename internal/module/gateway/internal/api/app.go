package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type AppAPI struct{}

func (a *AppAPI) Register(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "app register entrypoint reserved")
}

func (a *AppAPI) List(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "app list entrypoint reserved")
}

func (a *AppAPI) Get(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "app get entrypoint reserved")
}

func (a *AppAPI) Heartbeat(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "app heartbeat entrypoint reserved")
}

func (a *AppAPI) Enable(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "app enable entrypoint reserved")
}

func (a *AppAPI) Disable(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "app disable entrypoint reserved")
}

func (a *AppAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/apps")
	routes.Post("/register", a.Register)
	routes.Get("", a.List)
	routes.Get("/:id", a.Get)
	routes.Post("/heartbeat", a.Heartbeat)
	routes.Post("/:id/enable", a.Enable)
	routes.Post("/:id/disable", a.Disable)
	return routes
}
