package http

import (
	relaydto "cs-im-server/internal/module/relay/dto"
	"net/http"

	"cs-im-server/internal/module/relay/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type ChannelAPI struct {
	Service *service.RelayService
}

func NewChannelAPI(relayService *service.RelayService) *ChannelAPI {
	return &ChannelAPI{Service: relayService}
}

func (a *ChannelAPI) Create(ctx httpapi.Context) {
	var req relaydto.CreateChannelRequest
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

func (a *ChannelAPI) Get(ctx httpapi.Context) {
	channel, err := a.Service.GetChannel(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", channel)
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
	var req relaydto.UpdateChannelRequest
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
	routes.Get("/:id", a.Get)
	routes.Post("/:id/enable", a.Enable)
	routes.Post("/:id/disable", a.Disable)
	routes.Patch("/:id", a.Update)
	routes.Post("/:id/test", a.Test)
	return routes
}
