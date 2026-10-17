package api

import (
	"net/http"

	"cs-im-server/internal/module/gateway/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type ChannelAPI struct {
	Service *service.Service
}

func (a *ChannelAPI) Create(ctx httpapi.Context) {
	var req service.CreateChannelRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	channel, err := a.Service.CreateChannel(ctx.Context(), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", channel)
}

func (a *ChannelAPI) List(ctx httpapi.Context) {
	channels := a.Service.ListChannels(ctx.Context())
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", channels)
}

func (a *ChannelAPI) Enable(ctx httpapi.Context) {
	channel, err := a.Service.SetChannelStatus(ctx.Context(), ctx.Param("id"), true)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", channel)
}

func (a *ChannelAPI) Disable(ctx httpapi.Context) {
	channel, err := a.Service.SetChannelStatus(ctx.Context(), ctx.Param("id"), false)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", channel)
}

func (a *ChannelAPI) Update(ctx httpapi.Context) {
	var req service.UpdateChannelRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	channel, err := a.Service.UpdateChannel(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", channel)
}

func (a *ChannelAPI) Test(ctx httpapi.Context) {
	channel, err := a.Service.TestChannel(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", channel)
}

func (a *ChannelAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/channels")
	routes.Post("", a.Create)
	routes.Get("", a.List)
	routes.Post("/:id/enable", a.Enable)
	routes.Post("/:id/disable", a.Disable)
	routes.Patch("/:id", a.Update)
	routes.Post("/:id/test", a.Test)
	return routes
}
