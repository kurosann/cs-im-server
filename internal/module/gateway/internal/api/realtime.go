package api

import (
	"cs-im-server/internal/module/gateway/internal/runtime"
	"cs-im-server/internal/module/gateway/internal/service"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/sse"
	commonmodule "vane-common/module"
	logger "vane-common/pkg/logger"
)

type RealtimeAPI struct {
	Realtime      runtime.RealtimeSubscriber
	Conversations *service.ConversationService
}

func NewRealtimeAPI(realtime runtime.RealtimeSubscriber, conversations *service.ConversationService) *RealtimeAPI {
	return &RealtimeAPI{Realtime: realtime, Conversations: conversations}
}

func (a *RealtimeAPI) Stream(ctx httpapi.Context) {
	logger.Debugf(ctx.Context(), "gateway api %s %s: stream realtime events", ctx.Request().Method, ctx.Request().URL.Path)
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
