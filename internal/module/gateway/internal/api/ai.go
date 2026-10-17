package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type AIAPI struct{}

func (a *AIAPI) GetConfig(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "ai config get entrypoint reserved")
}

func (a *AIAPI) UpdateConfig(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "ai config update entrypoint reserved")
}

func (a *AIAPI) Executions(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "ai executions entrypoint reserved")
}

func (a *AIAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/ai")
	routes.Get("/config", a.GetConfig)
	routes.Put("/config", a.UpdateConfig)
	routes.Get("/executions", a.Executions)
	return routes
}
