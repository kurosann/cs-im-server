package http

import (
	"net/http"

	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/seqquery"
	"cs-im-server/pkg/sse"
	commonmodule "vane-common/module"
)

type RelayMessageAPI struct {
	Messages gatewaypkg.MessageEgress
	Realtime gatewaypkg.RealtimeSubscriber
}

func NewRelayMessageAPI(messages gatewaypkg.MessageEgress, realtime gatewaypkg.RealtimeSubscriber) *RelayMessageAPI {
	return &RelayMessageAPI{Messages: messages, Realtime: realtime}
}

func canUseRelay(principal iampkg.Principal) bool {
	if principal.HasKind(iampkg.PrincipalKindRelay) {
		return true
	}
	return principal.Kind == iampkg.PrincipalKindHuman && principal.HasScope(iampkg.ScopeRelayAccess)
}

func (a *RelayMessageAPI) relayID(ctx httpapi.Context) (string, bool) {
	principal, ok := iampkg.PrincipalFromContext(ctx.Context())
	if !ok || !canUseRelay(principal) {
		writeServiceError(ctx, iampkg.ErrPrincipalMissing)
		return "", false
	}
	return principal.AccountKey, true
}

func (a *RelayMessageAPI) Send(ctx httpapi.Context) {
	relayID, ok := a.relayID(ctx)
	if !ok {
		return
	}
	var req gatewaypkg.CreateMessageRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	message, err := a.Messages.CreateRelayMessage(ctx.Context(), relayID, req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *RelayMessageAPI) Stream(ctx httpapi.Context) {
	if _, ok := a.relayID(ctx); !ok {
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
			if !a.Messages.RelayEventAllowed(event) {
				continue
			}
			if err := sse.WriteEvent(ctx, event); err != nil {
				return
			}
		}
	}
}

func (a *RelayMessageAPI) History(ctx httpapi.Context) {
	if _, ok := a.relayID(ctx); !ok {
		return
	}
	filter, err := seqquery.FromHTTP(ctx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	page, err := a.Messages.RelayHistory(ctx.Context(), ctx.Param("conversationId"), filter)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", page)
}

func (a *RelayMessageAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/api/v1/relays")
	routes.Post("/messages", a.Send)
	routes.Get("/conversations/:conversationId/messages", a.History)
	routes.Get("/realtime", a.Stream)
	return routes
}
