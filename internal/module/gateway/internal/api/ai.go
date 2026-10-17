package api

import (
	"net/http"

	"cs-im-server/internal/module/gateway/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type AIAPI struct {
	Service *service.Service
}

func (a *AIAPI) GetConfig(ctx httpapi.Context) {
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", a.Service.GetAIConfig(ctx.Context()))
}

func (a *AIAPI) UpdateConfig(ctx httpapi.Context) {
	var req service.AIConfig
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", a.Service.UpdateAIConfig(ctx.Context(), req))
}

func (a *AIAPI) Executions(ctx httpapi.Context) {
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", a.Service.ListAIExecutions(ctx.Context()))
}

func (a *AIAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/ai")
	routes.Get("/config", a.GetConfig)
	routes.Put("/config", a.UpdateConfig)
	routes.Get("/executions", a.Executions)
	return routes
}
