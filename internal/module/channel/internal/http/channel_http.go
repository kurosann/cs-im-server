package http

import (
	"net/http"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/seqquery"
	commonmodule "vane-common/module"
)

type ChannelIngressAPI struct {
	Apps          gatewaypkg.AppAuthorizer
	Conversations gatewaypkg.ConversationIngress
	MessageSvc    gatewaypkg.MessageIngress
}

func NewChannelIngressAPI(appService gatewaypkg.AppAuthorizer, conversationService gatewaypkg.ConversationIngress, messageService gatewaypkg.MessageIngress) *ChannelIngressAPI {
	return &ChannelIngressAPI{
		Apps:          appService,
		Conversations: conversationService,
		MessageSvc:    messageService,
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
	return apps.AuthorizeAccount(appID, principal)
}

func (a *ChannelIngressAPI) SendMessage(ctx httpapi.Context) {
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	var req gatewaydto.CreateMessageRequest
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

func (a *ChannelIngressAPI) EnsureConversation(ctx httpapi.Context) {
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	var req gatewaydto.EnsureConversationRequest
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

func (a *ChannelIngressAPI) Heartbeat(ctx httpapi.Context) {
	principal, err := appPrincipal(ctx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	app, err := a.Apps.Heartbeat(ctx.Context(), principal, gatewaydto.HeartbeatRequest{
		AppID: ctx.Param("id"),
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
}

func (a *ChannelIngressAPI) GetConversation(ctx httpapi.Context) {
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
	if err := authorizeChannel(ctx, ctx.Param("id"), a.Apps); err != nil {
		writeServiceError(ctx, err)
		return
	}
	var req gatewaydto.BatchCreateMessagesRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	result, err := a.MessageSvc.CreateChannelMessages(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", result)
}

func (a *ChannelIngressAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/channels")
	routes.Post("/:id/conversations/ensure", a.EnsureConversation)
	routes.Post("/:id/heartbeat", a.Heartbeat)
	routes.Get("/:id/conversations/:conversationId", a.GetConversation)
	routes.Post("/:id/messages", a.SendMessage)
	routes.Get("/:id/conversations/:conversationId/messages", a.Messages)
	routes.Get("/:id/conversations/:conversationId/offset", a.MessageOffset)
	routes.Post("/:id/messages/batch", a.SendMessagesBatch)
	return routes
}
