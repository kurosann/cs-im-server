package api

import (
	"net/http"

	"cs-im-server/internal/module/gateway/service"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/seqquery"
	commonmodule "vane-common/module"
)

type MessageAPI struct {
	Service *service.MessageService
}

func NewMessageAPI(messageService *service.MessageService) *MessageAPI {
	return &MessageAPI{Service: messageService}
}

func (a *MessageAPI) Create(ctx httpapi.Context) {
	var req service.CreateMessageRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	message, err := a.Service.Create(ctx.Context(), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *MessageAPI) Get(ctx httpapi.Context) {
	message, err := a.Service.Get(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *MessageAPI) UpdateStatus(ctx httpapi.Context) {
	var req service.UpdateMessageStatusRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	message, err := a.Service.UpdateStatus(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *MessageAPI) History(ctx httpapi.Context) {
	filter, err := seqquery.FromHTTP(ctx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	page, err := a.Service.History(ctx.Context(), ctx.Param("id"), ctx.Param("conversationId"), filter)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", page)
}

func (a *MessageAPI) AdminRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/messages")
	routes.Post("", a.Create)
	routes.Get("/:id", a.Get)
	routes.Post("/:id/status", a.UpdateStatus)
	return routes
}

func (a *MessageAPI) AdminConversationRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/conversations")
	routes.Get("/:id/messages", a.ConversationMessages)
	routes.Get("/:id/messages/:conversationId", a.History)
	return routes
}

func (a *MessageAPI) ConversationMessages(ctx httpapi.Context) {
	filter, err := seqquery.FromHTTP(ctx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	messages := a.Service.List(ctx.Context(), ctx.Param("id"), filter.AfterSeq, filter.BeforeSeq, filter.Limit)
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", messages)
}
