package api

import (
	"net/http"

	"cs-im-server/internal/module/gateway/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

type MessageAPI struct {
	Service *service.Service
}

func (a *MessageAPI) Create(ctx httpapi.Context) {
	var req service.CreateMessageRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	message, err := a.Service.CreateMessage(ctx.Context(), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *MessageAPI) UpdateStatus(ctx httpapi.Context) {
	var req service.UpdateMessageStatusRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	message, err := a.Service.UpdateMessageStatus(ctx.Context(), ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", message)
}

func (a *MessageAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/messages")
	routes.Post("", a.Create)
	routes.Post("/:id/status", a.UpdateStatus)
	return routes
}
