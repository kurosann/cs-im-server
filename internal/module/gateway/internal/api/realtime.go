package api

import (
	"cs-im-server/internal/module/gateway/internal/service"
	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/sse"
	commonmodule "vane-common/module"
)

type RealtimeAPI struct {
	Realtime      gatewaypkg.RealtimeSubscriber
	Conversations *service.ConversationService
}

func NewRealtimeAPI(realtime gatewaypkg.RealtimeSubscriber, conversations *service.ConversationService) *RealtimeAPI {
	return &RealtimeAPI{Realtime: realtime, Conversations: conversations}
}

func (a *RealtimeAPI) Stream(ctx httpapi.Context) {
	principal, ok := iampkg.PrincipalFromContext(ctx.Context())
	if !ok {
		writeServiceError(ctx, iampkg.ErrPrincipalMissing)
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
			if !a.Conversations.AdminEventAllowed(principal, event) {
				continue
			}
			if err := sse.WriteEvent(ctx, event); err != nil {
				return
			}
		}
	}
}

func (a *RealtimeAPI) AdminRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/realtime")
	routes.Get("", a.Stream)
	return routes
}
