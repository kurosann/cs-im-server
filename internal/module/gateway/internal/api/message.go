package api

import (
	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"net/http"

	"cs-im-server/internal/module/gateway/internal/service"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/seqquery"
	commonmodule "vane-common/module"
	logger "vane-common/pkg/logger"
)

type MessageAPI struct {
	Service *service.MessageService
}

func NewMessageAPI(messageService *service.MessageService) *MessageAPI {
	return &MessageAPI{Service: messageService}
}

func (a *MessageAPI) Create(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "gateway api %s %s: create message", ctx.Request().Method, ctx.Request().URL.Path)
	var req gatewaydto.CreateMessageRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	message, err := a.Service.Create(ctx.Context(), req, true)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *MessageAPI) Get(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "gateway api %s %s: get message id=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	message, err := a.Service.Get(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *MessageAPI) UpdateStatus(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "gateway api %s %s: update message status id=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	var req gatewaydto.UpdateMessageStatusRequest
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

func (a *MessageAPI) Retry(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "gateway api %s %s: retry message id=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	message, err := a.Service.RetryMessage(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *MessageAPI) ReverseHistory(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "gateway api %s %s: reverse conversation history id=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	var req gatewaydto.BatchCreateMessagesRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	queryFilter, err := seqquery.FromHTTP(ctx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	page, createdCount, err := a.Service.ReverseHistory(ctx.Context(), ctx.Param("id"), req, gatewaydto.MessageHistoryFilter{
		AfterSeq:  queryFilter.AfterSeq,
		BeforeSeq: queryFilter.BeforeSeq,
		Limit:     queryFilter.Limit,
		Order:     queryFilter.Order,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", map[string]any{
		"page":         page,
		"createdCount": createdCount,
	})
}

func (a *MessageAPI) AdminRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/messages")
	routes.Post("", a.Create)
	routes.Get("/:id", a.Get)
	routes.Post("/:id/status", a.UpdateStatus)
	routes.Post("/:id/retry", a.Retry)
	return routes
}

func (a *MessageAPI) AdminConversationRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/conversations")
	routes.Get("/:id/messages", a.ConversationMessages)
	routes.Post("/:id/channel-history", a.ReverseHistory)
	routes.Post("/:id/history-requests", a.RequestHistory)
	return routes
}

func (a *MessageAPI) ConversationMessages(ctx httpapi.Context) {
	filter, err := seqquery.FromHTTP(ctx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	logger.Debugf(ctx.Context(), "gateway api %s %s: list conversation messages id=%s afterSeq=%d beforeSeq=%d limit=%d", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"), filter.AfterSeq, filter.BeforeSeq, filter.Limit)
	messages := a.Service.List(ctx.Context(), ctx.Param("id"), filter.AfterSeq, filter.BeforeSeq, filter.Limit)
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", messages)
}
