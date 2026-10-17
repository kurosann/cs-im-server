package http

import (
	"net/http"

	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/seqquery"
	"cs-im-server/pkg/sse"
	commonmodule "vane-common/module"
)

type ChannelGatewayHTTPAPI struct {
	Accounts      *iampkg.AccountService
	Apps          *gatewaypkg.AppService
	Conversations *gatewaypkg.ConversationService
	MessageSvc    *gatewaypkg.MessageService
	Realtime      *gatewaypkg.RealtimeService
}

func NewChannelGatewayHTTPAPI(accountService *iampkg.AccountService, appService *gatewaypkg.AppService, conversationService *gatewaypkg.ConversationService, messageService *gatewaypkg.MessageService, realtimeService *gatewaypkg.RealtimeService) *ChannelGatewayHTTPAPI {
	return &ChannelGatewayHTTPAPI{
		Accounts:      accountService,
		Apps:          appService,
		Conversations: conversationService,
		MessageSvc:    messageService,
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

func authorizeChannel(ctx httpapi.Context, appID string, apps *gatewaypkg.AppService) error {
	principal, err := appPrincipal(ctx)
	if err != nil {
		return err
	}
	return apps.AuthorizeAccount(appID, principal)
}

func (a *ChannelGatewayHTTPAPI) SendMessage(ctx httpapi.Context) {
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	var req gatewaypkg.CreateMessageRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	message, err := a.MessageSvc.CreateChannelMessage(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *ChannelGatewayHTTPAPI) EnsureConversation(ctx httpapi.Context) {
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	var req gatewaypkg.EnsureConversationRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	conversation, created, err := a.Conversations.Ensure(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", map[string]any{"conversation": conversation, "created": created})
}

func (a *ChannelGatewayHTTPAPI) Messages(ctx httpapi.Context) {
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

func (a *ChannelGatewayHTTPAPI) Stream(ctx httpapi.Context) {
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
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
	for {
		select {
		case <-ctx.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if !a.MessageSvc.ChannelEventAllowed(ctx.Param("id"), event) {
				continue
			}
			if err := sse.WriteEvent(ctx, event); err != nil {
				return
			}
		}
	}
}

func (a *ChannelGatewayHTTPAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/channels")
	routes.Post("/:id/conversations/ensure", a.EnsureConversation)
	routes.Post("/:id/messages", a.SendMessage)
	routes.Get("/:id/conversations/:conversationId/messages", a.Messages)
	routes.Get("/:id/realtime", a.Stream)
	return routes
}
