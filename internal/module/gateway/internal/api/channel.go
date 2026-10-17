package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type ChannelAPI struct{}

func (a *ChannelAPI) Create(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "channel create entrypoint reserved")
}

func (a *ChannelAPI) List(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "channel list entrypoint reserved")
}

func (a *ChannelAPI) Enable(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "channel enable entrypoint reserved")
}

func (a *ChannelAPI) Disable(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "channel disable entrypoint reserved")
}

func (a *ChannelAPI) Update(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "channel update entrypoint reserved")
}

func (a *ChannelAPI) Test(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "channel test entrypoint reserved")
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
