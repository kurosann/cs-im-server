package api

import (
	"net/http"

	"cs-im-server/internal/module/gateway/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type CustomerAPI struct {
	Service *service.CustomerService
}

func NewCustomerAPI(customerService *service.CustomerService) *CustomerAPI {
	return &CustomerAPI{Service: customerService}
}

func (a *CustomerAPI) Create(ctx httpapi.Context) {
	var req service.CreateCustomerRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	customer, err := a.Service.Create(ctx.Context(), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", customer)
}

func (a *CustomerAPI) Get(ctx httpapi.Context) {
	customer, err := a.Service.Get(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", customer)
}

func (a *CustomerAPI) AddIdentity(ctx httpapi.Context) {
	var req service.AddIdentityRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	customer, err := a.Service.AddIdentity(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", customer)
}

func (a *CustomerAPI) Merge(ctx httpapi.Context) {
	var req service.MergeCustomerRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	customer, err := a.Service.Merge(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", customer)
}

func (a *CustomerAPI) AdminRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/customers")
	routes.Post("", a.Create)
	routes.Get("/:id", a.Get)
	routes.Post("/:id/identities", a.AddIdentity)
	routes.Post("/:id/merge", a.Merge)
	return routes
}
