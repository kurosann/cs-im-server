package api

import (
	"net/http"

	"cs-im-server/internal/module/gateway/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type ConversationAPI struct {
	Service *service.Service
}

func (a *ConversationAPI) Create(ctx httpapi.Context) {
	var req service.CreateConversationRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	conversation, err := a.Service.CreateConversation(ctx.Context(), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", conversation)
}

func (a *ConversationAPI) List(ctx httpapi.Context) {
	conversations := a.Service.ListConversations(ctx.Context(), ctx.Query("appId"), ctx.Query("customerId"), ctx.Query("status"))
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", conversations)
}

func (a *ConversationAPI) Get(ctx httpapi.Context) {
	conversation, err := a.Service.GetConversation(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", conversation)
}

func (a *ConversationAPI) Assign(ctx httpapi.Context) {
	var req service.AssignConversationRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	conversation, err := a.Service.AssignConversation(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", conversation)
}

func (a *ConversationAPI) Close(ctx httpapi.Context) {
	var req service.CloseConversationRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	conversation, err := a.Service.CloseConversation(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", conversation)
}

func (a *ConversationAPI) Forward(ctx httpapi.Context) {
	var req service.ForwardConversationRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	delivery, err := a.Service.ForwardConversation(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", delivery)
}

func (a *ConversationAPI) Messages(ctx httpapi.Context) {
	messages := a.Service.ConversationMessages(ctx.Context(), ctx.Param("id"))
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", messages)
}

func (a *ConversationAPI) Deliveries(ctx httpapi.Context) {
	deliveries := a.Service.ConversationDeliveries(ctx.Context(), ctx.Param("id"))
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", deliveries)
}

func (a *ConversationAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/conversations")
	routes.Post("", a.Create)
	routes.Get("", a.List)
	routes.Get("/:id", a.Get)
	routes.Post("/:id/assign", a.Assign)
	routes.Post("/:id/close", a.Close)
	routes.Post("/:id/forward", a.Forward)
	routes.Get("/:id/messages", a.Messages)
	routes.Get("/:id/deliveries", a.Deliveries)
	return routes
}
