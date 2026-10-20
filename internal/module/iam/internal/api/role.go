package api

import (
	iamdto "cs-im-server/internal/module/iam/dto"
	"net/http"

	"cs-im-server/internal/module/iam/internal/service"
	pkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/httpapi"

	commonmodule "vane-common/module"
)

type RoleAPI struct {
	Roles *service.RoleService
}

func NewRoleAPI(roles *service.RoleService) *RoleAPI {
	return &RoleAPI{Roles: roles}
}

type RoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Scopes      []string `json:"scopes"`
}

func (a *RoleAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/roles")
	routes.Get("", a.List)
	routes.Post("", a.Create)
	routes.Put("/:roleKey", a.Update)
	routes.Delete("/:roleKey", a.Delete)
	return routes
}

func (a *RoleAPI) principal(ctx httpapi.Context) (pkg.Principal, bool) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
	}
	return principal, ok
}

func (a *RoleAPI) List(ctx httpapi.Context) {
	principal, ok := a.principal(ctx)
	if !ok {
		return
	}
	items, err := a.Roles.List(ctx.Context(), principal)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", items)
}

func (a *RoleAPI) Create(ctx httpapi.Context) {
	principal, ok := a.principal(ctx)
	if !ok {
		return
	}
	var req RoleRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	item, err := a.Roles.Create(ctx.Context(), principal, iamdto.CreateRoleRequest(req))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}

func (a *RoleAPI) Update(ctx httpapi.Context) {
	principal, ok := a.principal(ctx)
	if !ok {
		return
	}
	var req RoleRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	item, err := a.Roles.Update(ctx.Context(), principal, ctx.Param("roleKey"), iamdto.UpdateRoleRequest(req))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}

func (a *RoleAPI) Delete(ctx httpapi.Context) {
	principal, ok := a.principal(ctx)
	if !ok {
		return
	}
	if err := a.Roles.Delete(ctx.Context(), principal, ctx.Param("roleKey")); err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", "deleted")
}
