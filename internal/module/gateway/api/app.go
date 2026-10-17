package api

import (
	"context"
	"net/http"

	"cs-im-server/internal/module/gateway/service"
	iampkg "cs-im-server/internal/module/iam/pkg"
	"cs-im-server/pkg/httpapi"
	"cs-im-server/pkg/seqquery"
	commonmodule "vane-common/module"
)

type AppAPI struct {
	Service         *service.AppService
	MessagesService MessageHistoryService
}

type MessageHistoryService interface {
	HistoryForApp(ctx context.Context, appID, conversationID string, filter seqquery.Filter) (any, error)
}

func NewAppAPI(appService *service.AppService, messageService MessageHistoryService) *AppAPI {
	return &AppAPI{Service: appService, MessagesService: messageService}
}

func (a *AppAPI) Register(ctx httpapi.Context) {
	principal, ok := iampkg.PrincipalFromContext(ctx.Context())
	if !ok {
		writeServiceError(ctx, iampkg.ErrPrincipalMissing)
		return
	}
	var req service.RegisterAppRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	app, err := a.Service.Register(ctx.Context(), principal, req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
}

func (a *AppAPI) List(ctx httpapi.Context) {
	apps := a.Service.List(ctx.Context())
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", apps)
}

func (a *AppAPI) Get(ctx httpapi.Context) {
	app, err := a.Service.Get(ctx.Context(), ctx.Param("id"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
}

func (a *AppAPI) Heartbeat(ctx httpapi.Context) {
	var req service.HeartbeatRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	app, err := a.Service.Heartbeat(ctx.Context(), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
}

func (a *AppAPI) Enable(ctx httpapi.Context) {
	app, err := a.Service.SetStatus(ctx.Context(), ctx.Param("id"), true)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
}

func (a *AppAPI) Disable(ctx httpapi.Context) {
	app, err := a.Service.SetStatus(ctx.Context(), ctx.Param("id"), false)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", app)
}

func (a *AppAPI) Messages(ctx httpapi.Context) {
	filter, err := seqquery.FromHTTP(ctx)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	page, err := a.MessagesService.HistoryForApp(ctx.Context(), ctx.Param("id"), ctx.Param("conversationId"), filter)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", page)
}

func (a *AppAPI) AdminRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/apps")
	routes.Post("/register", a.Register)
	routes.Post("/heartbeat", a.Heartbeat)
	routes.Get("", a.List)
	routes.Get("/:id", a.Get)
	routes.Get("/:id/conversations/:conversationId/messages", a.Messages)
	routes.Post("/:id/enable", a.Enable)
	routes.Post("/:id/disable", a.Disable)
	return routes
}
