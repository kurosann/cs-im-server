package api

import (
	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"net/http"

	"cs-im-server/internal/module/gateway/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
	logger "vane-common/pkg/logger"
)

type CustomerAPI struct {
	Service       *service.CustomerService
	Conversations *service.ConversationService
}

func NewCustomerAPI(customerService *service.CustomerService, conversations *service.ConversationService) *CustomerAPI {
	return &CustomerAPI{Service: customerService, Conversations: conversations}
}

func (a *CustomerAPI) Create(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "gateway api %s %s: create customer", ctx.Request().Method, ctx.Request().URL.Path)
	var req gatewaydto.CreateCustomerRequest
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
	logger.Debugf(ctx.Context(), "gateway api %s %s: get customer id=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	customer, err := a.Service.Get(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", customer)
}

func (a *CustomerAPI) AddIdentity(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "gateway api %s %s: add customer identity id=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	var req gatewaydto.AddIdentityRequest
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
	logger.Debugf(ctx.Context(), "gateway api %s %s: merge customer id=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	var req gatewaydto.MergeCustomerRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	customer, err := a.Service.Merge(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	if err := a.Conversations.RetargetCustomer(req.SourceCustomerID, ctx.Param("id")); err != nil {
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
