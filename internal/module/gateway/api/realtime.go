package api

import (
	"cs-im-server/internal/module/gateway/service"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/sse"
	commonmodule "vane-common/module"
)

type RealtimeAPI struct {
	Service *service.RealtimeService
}

func NewRealtimeAPI(realtimeService *service.RealtimeService) *RealtimeAPI {
	return &RealtimeAPI{Service: realtimeService}
}

func (a *RealtimeAPI) Subscribe(ctx httpapi.Context) {
	events, closeFn, err := a.Service.Subscribe(ctx.Context())
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
			if err := sse.WriteEvent(ctx, event); err != nil {
				return
			}
		}
	}
}

func (a *RealtimeAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("")
	routes.Get("/realtime", a.Subscribe)
	return routes
}
