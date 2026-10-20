package api

import (
	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"net/http"
	"strconv"

	"cs-im-server/internal/module/gateway/internal/service"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type ConversationAPI struct {
	Service *service.ConversationService
}

func NewConversationAPI(conversationService *service.ConversationService) *ConversationAPI {
	return &ConversationAPI{Service: conversationService}
}

func (a *ConversationAPI) Create(ctx httpapi.Context) {
	var req gatewaydto.CreateConversationRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	conversation, err := a.Service.Create(ctx.Context(), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", conversation)
}

func (a *ConversationAPI) List(ctx httpapi.Context) {
	principal, ok := iampkg.PrincipalFromContext(ctx.Context())
	if !ok {
		writeServiceError(ctx, iampkg.ErrPrincipalMissing)
		return
	}
	page, _ := strconv.Atoi(ctx.Query("page"))
	pageSize, _ := strconv.Atoi(ctx.Query("pageSize"))
	pageData, err := a.Service.ListForAdmin(ctx.Context(), principal, service.ConversationFilter{
		Keyword:  ctx.Query("keyword"),
		AppID:    ctx.Query("appId"),
		Status:   ctx.Query("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", pageData)
}

func (a *ConversationAPI) Get(ctx httpapi.Context) {
	conversation, err := a.Service.Get(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", conversation)
}

func (a *ConversationAPI) Assign(ctx httpapi.Context) {
	var req gatewaydto.AssignConversationRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	conversation, err := a.Service.Assign(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", conversation)
}

func (a *ConversationAPI) Close(ctx httpapi.Context) {
	var req gatewaydto.CloseConversationRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	conversation, err := a.Service.Close(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", conversation)
}

func (a *ConversationAPI) Forward(ctx httpapi.Context) {
	var req gatewaydto.ForwardConversationRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	delivery, err := a.Service.Forward(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", delivery)
}

func (a *ConversationAPI) Deliveries(ctx httpapi.Context) {
	deliveries := a.Service.Deliveries(ctx.Context(), ctx.Param("id"))
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", deliveries)
}

func (a *ConversationAPI) MarkRead(ctx httpapi.Context) {
	markedCount, err := a.Service.MarkRead(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", markedCount)
}

func (a *ConversationAPI) AdminRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/conversations")
	routes.Post("", a.Create)
	routes.Get("", a.List)
	routes.Get("/:id", a.Get)
	routes.Post("/:id/assign", a.Assign)
	routes.Post("/:id/close", a.Close)
	routes.Post("/:id/forward", a.Forward)
	routes.Post("/:id/read", a.MarkRead)
	routes.Get("/:id/deliveries", a.Deliveries)
	return routes
}
