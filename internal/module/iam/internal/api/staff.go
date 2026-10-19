package api

import (
	"net/http"

	"cs-im-server/internal/module/iam/internal/service"
	pkg "cs-im-server/internal/module/iam/pkg"
	"cs-im-server/pkg/httpapi"

	commonmodule "vane-common/module"
)

type StaffAPI struct {
	Staff *service.StaffService
}

func NewStaffAPI(staff *service.StaffService) *StaffAPI {
	return &StaffAPI{Staff: staff}
}

type StaffRoleRequest struct {
	Role string `json:"role"`
}

type StaffStatusRequest struct {
	Status string `json:"status"`
}

func (a *StaffAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/staff")
	routes.Get("", a.List)
	routes.Post("", a.Create)
	routes.Post("/:userKey/role", a.SetRole)
	routes.Post("/:userKey/status", a.SetStatus)
	return routes
}

func (a *StaffAPI) List(ctx httpapi.Context) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	items, err := a.Staff.List(ctx.Context(), principal)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", items)
}

func (a *StaffAPI) Create(ctx httpapi.Context) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	var req service.CreateStaffRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	result, err := a.Staff.Create(ctx.Context(), principal, req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", result)
}

func (a *StaffAPI) SetRole(ctx httpapi.Context) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	var req StaffRoleRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	item, err := a.Staff.SetRole(ctx.Context(), principal, ctx.Param("userKey"), req.Role)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}

func (a *StaffAPI) SetStatus(ctx httpapi.Context) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	var req StaffStatusRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	item, err := a.Staff.SetStatus(ctx.Context(), principal, ctx.Param("userKey"), req.Status)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}
