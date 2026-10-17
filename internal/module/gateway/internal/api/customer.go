package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
)

type CustomerAPI struct{}

func (a *CustomerAPI) Create(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "customer create entrypoint reserved")
}

func (a *CustomerAPI) Get(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "customer get entrypoint reserved")
}
