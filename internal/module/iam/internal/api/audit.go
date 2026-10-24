package api

import (
	iamdto "cs-im-server/internal/module/iam/dto"
	"net/http"
	"strconv"

	"cs-im-server/internal/module/iam/internal/service"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	"cs-im-server/pkg/httpapi"

	commonmodule "vane-common/module"
)

type AuditAPI struct {
	Audit *service.AuditService
}

func NewAuditAPI(audit *service.AuditService) *AuditAPI {
	return &AuditAPI{Audit: audit}
}

func (a *AuditAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/audit-logs")
	routes.Get("", a.List)
	return routes
}

func (a *AuditAPI) List(ctx httpapi.Context) {
	principal, ok := iamprincipal.FromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", iamprincipal.ErrMissing.Error())
		return
	}
	page, _ := strconv.Atoi(ctx.Query("page"))
	pageSize, _ := strconv.Atoi(ctx.Query("pageSize"))
	items, err := a.Audit.List(ctx.Context(), principal, iamdto.AuditFilter{
		Keyword:  ctx.Query("keyword"),
		Action:   ctx.Query("action"),
		Result:   ctx.Query("result"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", items)
}
