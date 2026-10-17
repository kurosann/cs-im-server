package api

import (
	"net/http"

	"cs-im-server/internal/module/gateway/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type AppAPI struct {
	Service *service.Service
}

func (a *AppAPI) Register(ctx httpapi.Context) {
	var req service.RegisterAppRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	app, err := a.Service.RegisterApp(ctx.Context(), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
}

func (a *AppAPI) List(ctx httpapi.Context) {
	apps := a.Service.ListApps(ctx.Context())
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", apps)
}

func (a *AppAPI) Get(ctx httpapi.Context) {
	app, err := a.Service.GetApp(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
}

func (a *AppAPI) Heartbeat(ctx httpapi.Context) {
	var req service.HeartbeatRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	app, err := a.Service.Heartbeat(ctx.Context(), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
}

func (a *AppAPI) Enable(ctx httpapi.Context) {
	app, err := a.Service.SetAppStatus(ctx.Context(), ctx.Param("id"), true)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
}

func (a *AppAPI) Disable(ctx httpapi.Context) {
	app, err := a.Service.SetAppStatus(ctx.Context(), ctx.Param("id"), false)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
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
