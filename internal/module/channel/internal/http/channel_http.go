package http

import (
	"net/http"

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

func (a *ChannelIngressAPI) EnsureConversation(ctx httpapi.Context) {
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

func (a *ChannelIngressAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/channels")
	routes.Post("/:id/conversations/ensure", a.EnsureConversation)
	routes.Get("/:id/conversations/:conversationId", a.GetConversation)
	routes.Post("/:id/messages", a.SendMessage)
	routes.Get("/:id/conversations/:conversationId/messages", a.Messages)
	return routes
}
