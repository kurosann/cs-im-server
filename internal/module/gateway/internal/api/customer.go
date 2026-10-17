package api

import (
	"net/http"

	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type CustomerAPI struct{}

func (a *CustomerAPI) Create(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "customer create entrypoint reserved")
}

func (a *CustomerAPI) Get(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "customer get entrypoint reserved")
}

func (a *CustomerAPI) AddIdentity(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "customer identity add entrypoint reserved")
}

func (a *CustomerAPI) Merge(ctx httpapi.Context) {
	httpapi.WriteError(ctx, http.StatusNotImplemented, "NOT_IMPLEMENTED", "customer merge entrypoint reserved")
}

func (a *CustomerAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/customers")
	routes.Post("", a.Create)
	routes.Get("/:id", a.Get)
	routes.Post("/:id/identities", a.AddIdentity)
	routes.Post("/:id/merge", a.Merge)
	return routes
}
