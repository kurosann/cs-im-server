package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
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
