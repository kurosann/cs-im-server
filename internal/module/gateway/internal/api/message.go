package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type MessageAPI struct{}

func (a *MessageAPI) Create(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "message create entrypoint reserved")
}

func (a *MessageAPI) UpdateStatus(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "message status update entrypoint reserved")
}

func (a *MessageAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/messages")
	routes.Post("", a.Create)
	routes.Post("/:id/status", a.UpdateStatus)
	return routes
}
