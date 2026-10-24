package api

import (
	iamdto "cs-im-server/internal/module/iam/dto"
	"net/http"

	"cs-im-server/internal/module/iam/internal/service"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
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

type StaffUpdateRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

func (a *StaffAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/staff")
	routes.Get("", a.List)
	routes.Post("", a.Create)
	routes.Put("/:userKey", a.Update)
	routes.Post("/:userKey/role", a.SetRole)
	routes.Post("/:userKey/status", a.SetStatus)
	routes.Post("/:userKey/password/reset", a.ResetPassword)
	routes.Post("/:userKey/logout", a.ForceLogout)
	return routes
}

func (a *StaffAPI) Update(ctx httpapi.Context) {
	principal, ok := iamprincipal.FromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", iamprincipal.ErrMissing.Error())
		return
	}
	var req StaffUpdateRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	item, err := a.Staff.Update(ctx.Context(), principal, ctx.Param("userKey"), iamdto.UpdateStaffRequest(req))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}

func (a *StaffAPI) List(ctx httpapi.Context) {
	principal, ok := iamprincipal.FromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", iamprincipal.ErrMissing.Error())
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
	principal, ok := iamprincipal.FromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", iamprincipal.ErrMissing.Error())
		return
	}
	var req iamdto.CreateStaffRequest
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
	principal, ok := iamprincipal.FromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", iamprincipal.ErrMissing.Error())
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
	principal, ok := iamprincipal.FromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", iamprincipal.ErrMissing.Error())
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

func (a *StaffAPI) ResetPassword(ctx httpapi.Context) {
	principal, ok := iamprincipal.FromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", iamprincipal.ErrMissing.Error())
		return
	}
	result, err := a.Staff.ResetPassword(ctx.Context(), principal, ctx.Param("userKey"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", result)
}

func (a *StaffAPI) ForceLogout(ctx httpapi.Context) {
	principal, ok := iamprincipal.FromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", iamprincipal.ErrMissing.Error())
		return
	}
	if err := a.Staff.ForceLogout(ctx.Context(), principal, ctx.Param("userKey")); err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", "logged_out")
}
