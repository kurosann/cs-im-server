package http

import (
	"net/http"

	channeldto "cs-im-server/internal/module/channel/dto"
	gatewaydto "cs-im-server/internal/module/gateway/dto"
	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/seqquery"
	"cs-im-server/pkg/sse"
	commonmodule "vane-common/module"
	logger "vane-common/pkg/logger"
)

type ChannelIngressAPI struct {
	Apps          gatewaypkg.AppAuthorizer
	Conversations gatewaypkg.ConversationIngress
	MessageSvc    gatewaypkg.MessageIngress
	HistorySvc    gatewaypkg.MessageEgress
	Realtime      gatewaypkg.RealtimeSubscriber
}

func NewChannelIngressAPI(appService gatewaypkg.AppAuthorizer, conversationService gatewaypkg.ConversationIngress, messageService gatewaypkg.MessageIngress, realtimeService gatewaypkg.RealtimeSubscriber) *ChannelIngressAPI {
	return &ChannelIngressAPI{
		Apps:          appService,
		Conversations: conversationService,
		MessageSvc:    messageService,
		HistorySvc:    messageService.(gatewaypkg.MessageEgress),
		Realtime:      realtimeService,
	}
}

func appPrincipal(ctx httpapi.Context) (iampkg.Principal, error) {
	principal, ok := iampkg.PrincipalFromContext(ctx.Context())
	if !ok {
		return iampkg.Principal{}, iampkg.ErrPrincipalMissing
	}
	return principal, nil
}

func authorizeChannel(ctx httpapi.Context, appID string, apps gatewaypkg.AppAuthorizer) error {
	principal, err := appPrincipal(ctx)
	if err != nil {
		return err
	}
	if err := apps.AuthorizeAccount(appID, principal); err != nil {
		return err
	}
	apps.MarkConnected(appID)
	return nil
}

func (a *ChannelIngressAPI) SendMessage(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "channel http %s %s: send message appID=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	var req channeldto.CreateMessageRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := req.Validate(); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	message, err := a.MessageSvc.CreateChannelMessage(ctx.Context(), ctx.Param("id"), channeldto.CreateMessageRequestToGateway(req))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *ChannelIngressAPI) EnsureConversation(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "channel http %s %s: ensure conversation appID=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	var req channeldto.EnsureConversationRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	conversation, created, err := a.Conversations.Ensure(ctx.Context(), ctx.Param("id"), channeldto.EnsureConversationToGateway(req))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", map[string]any{"conversation": conversation, "created": created})
}

func (a *ChannelIngressAPI) MessageStatus(ctx httpapi.Context) {
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	var req gatewaydto.UpdateMessageStatusRequest
	if err := ctx.BindJSON(&req); err != nil {
		writeServiceError(ctx, err)
		return
	}
	message, err := a.MessageSvc.UpdateChannelMessageStatus(
		ctx.Context(), ctx.Param("id"), ctx.Param("messageId"), req,
	)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *ChannelIngressAPI) Heartbeat(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "channel http %s %s: heartbeat appID=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	principal, err := appPrincipal(ctx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	app, err := a.Apps.Heartbeat(ctx.Context(), principal, channeldto.HeartbeatToGateway(channeldto.HeartbeatRequest{
		AppID: ctx.Param("id"),
	}))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
}

func (a *ChannelIngressAPI) GetConversation(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "channel http %s %s: get conversation appID=%s conversationID=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"), ctx.Param("conversationId"))
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	conversation, err := a.Conversations.Get(ctx.Context(), ctx.Param("conversationId"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", conversation)
}

func (a *ChannelIngressAPI) Messages(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "channel http %s %s: list conversation messages appID=%s conversationID=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"), ctx.Param("conversationId"))
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	filter, err := seqquery.FromHTTP(ctx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	page, err := a.MessageSvc.History(ctx.Context(), ctx.Param("id"), ctx.Param("conversationId"), filter)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", page)
}

func (a *ChannelIngressAPI) MessageOffset(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "channel http %s %s: get conversation offset appID=%s conversationID=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"), ctx.Param("conversationId"))
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	offset, err := a.MessageSvc.Offset(ctx.Context(), ctx.Param("id"), ctx.Param("conversationId"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", offset)
}

func (a *ChannelIngressAPI) SendMessagesBatch(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "channel http %s %s: send messages batch appID=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	var req channeldto.BatchCreateMessagesRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	result, err := a.MessageSvc.CreateChannelMessages(ctx.Context(), ctx.Param("id"), channeldto.BatchCreateMessagesToGateway(req))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", result)
}

func (a *ChannelIngressAPI) PendingHistoryRequests(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "channel http %s %s: get pending history requests appID=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	request, ok, err := a.MessageSvc.PendingHistoryRequest(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	if !ok {
		httpapi.WriteJSON(ctx, http.StatusOK, "OK", channeldto.HistoryRequest{})
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", channeldto.HistoryRequestFromGateway(request))
}

func (a *ChannelIngressAPI) Stream(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "channel http %s %s: stream realtime events appID=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	appID := ctx.Param("id")
	if err := authorizeChannel(ctx, appID, a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	events, closeFn, err := a.Realtime.Subscribe(ctx.Context())
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	defer closeFn()
	sse.WriteHeaders(ctx)
	ctx.Flush()
	for {
		select {
		case <-ctx.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if !a.MessageSvc.ChannelEventAllowed(appID, event) {
				continue
			}
			if err := sse.WriteEvent(ctx, event); err != nil {
				return
			}
		}
	}
}

func (a *ChannelIngressAPI) StreamHistoryRequests(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "channel http %s %s: stream history requests appID=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"))
	appID := ctx.Param("id")
	if err := authorizeChannel(ctx, appID, a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	events, closeFn, err := a.Realtime.Subscribe(ctx.Context())
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	defer closeFn()
	sse.WriteHeaders(ctx)
	if request, found, err := a.MessageSvc.PendingHistoryRequest(ctx.Context(), appID); err == nil && found {
		if err := sse.WriteEvent(ctx, gatewaypkg.RealtimeEvent{
			Type: "history.request.created",
			ID:   request.RequestID,
			Data: request,
		}); err != nil {
			return
		}
		ctx.Flush()
	}
	for {
		select {
		case <-ctx.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.Type != "history.request.created" {
				continue
			}
			request, ok := event.Data.(gatewaydto.HistoryRequest)
			if !ok || request.AppID != appID {
				continue
			}
			if err := sse.WriteEvent(ctx, event); err != nil {
				return
			}
		}
	}
}

func (a *ChannelIngressAPI) UpdateHistoryRequestStatus(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "channel http %s %s: update history request status appID=%s requestID=%s", ctx.Request().Method, ctx.Request().URL.Path, ctx.Param("id"), ctx.Param("requestId"))
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	var req channeldto.UpdateHistoryRequestStatusInput
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	historyStatus := req.Status
	if historyStatus == "" {
		historyStatus = channeldto.HistoryRequestStatusCompleted
	}
	if !historyStatus.Valid() {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "status must be completed or failed")
		return
	}

	var err error
	var gatewayRequest gatewaydto.HistoryRequest
	if historyStatus == channeldto.HistoryRequestStatusCompleted {
		gatewayRequest, err = a.HistorySvc.CompleteHistoryRequest(ctx.Context(), ctx.Param("id"), channeldto.UpdateHistoryRequestStatusToGateway(req))
	} else {
		gatewayRequest, err = a.HistorySvc.FailHistoryRequest(ctx.Context(), ctx.Param("id"), channeldto.UpdateHistoryRequestStatusToGateway(req))
	}
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	request := channeldto.HistoryRequestFromGateway(gatewayRequest)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", request)
}

func (a *ChannelIngressAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/channels")
	routes.Post("/:id/conversations/ensure", a.EnsureConversation)
	routes.Post("/:id/heartbeat", a.Heartbeat)
	routes.Get("/:id/realtime", a.Stream)
	routes.Get("/:id/conversations/:conversationId", a.GetConversation)
	routes.Post("/:id/messages", a.SendMessage)
	routes.Get("/:id/conversations/:conversationId/messages", a.Messages)
	routes.Get("/:id/conversations/:conversationId/offset", a.MessageOffset)
	routes.Post("/:id/messages/batch", a.SendMessagesBatch)
	routes.Post("/:id/messages/:messageId/status", a.MessageStatus)
	routes.Get("/:id/history-requests/pending", a.PendingHistoryRequests)
	routes.Get("/:id/control", a.StreamHistoryRequests)
	routes.Post("/:id/history-requests/:requestId/status", a.UpdateHistoryRequestStatus)
	return routes
}
