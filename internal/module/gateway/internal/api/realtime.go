package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cs-im-server/internal/module/gateway/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type RealtimeAPI struct {
	Service *service.Service
}

func (a *RealtimeAPI) Subscribe(ctx httpapi.Context) {
	events, closeFn, err := a.Service.Subscribe(ctx.Context())
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	defer closeFn()

	ctx.SetHeader("Content-Type", "text/event-stream")
	ctx.SetHeader("Cache-Control", "no-cache")
	ctx.SetHeader("Connection", "keep-alive")
	_ = ctx.SetWriteDeadline(time.Time{})
	ctx.Status(http.StatusOK)
	for {
		select {
		case <-ctx.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := writeSSE(ctx, event); err != nil {
				return
			}
		}
	}
}

func writeSSE(ctx httpapi.Context, event service.RealtimeEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(ctx, "data: %s\n\n", payload); err != nil {
		return err
	}
	return ctx.Flush()
}

func (a *RealtimeAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("")
	routes.Get("/realtime", a.Subscribe)
	return routes
}
