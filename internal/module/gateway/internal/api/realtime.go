package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type RealtimeAPI struct{}

func (a *RealtimeAPI) Subscribe(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "realtime entrypoint reserved")
}

func (a *RealtimeAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("")
	routes.Get("/realtime", a.Subscribe)
	return routes
}
