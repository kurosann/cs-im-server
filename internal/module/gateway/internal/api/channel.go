package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
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
