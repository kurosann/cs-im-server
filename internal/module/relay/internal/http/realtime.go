package http

import (
	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/sse"
	commonmodule "vane-common/module"
)

type RealtimeAPI struct {
	Apps       *gatewaypkg.AppService
	MessageSvc *gatewaypkg.MessageService
	Realtime   *gatewaypkg.RealtimeService
}

func NewRealtimeAPI(appService *gatewaypkg.AppService, messageService *gatewaypkg.MessageService, realtimeService *gatewaypkg.RealtimeService) *RealtimeAPI {
	return &RealtimeAPI{Apps: appService, MessageSvc: messageService, Realtime: realtimeService}
}

func (a *RealtimeAPI) Stream(ctx httpapi.Context) {
	principal, ok := iampkg.PrincipalFromContext(ctx.Context())
	if !ok {
		writeServiceError(ctx, iampkg.ErrPrincipalMissing)
		return
	}
	if err := a.Apps.AuthorizeAccount(ctx.Param("id"), principal); err != nil {
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

func (a *RealtimeAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/channels")
	routes.Get("/:id/realtime", a.Stream)
	return routes
}
