package http

import (
	"net/http"
	"strconv"
	"strings"

	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/internal/module/relay/internal/service"
	"cs-im-server/pkg/httpapi"
	commonmodule "vane-common/module"
)

func (a *AIAPI) ListEmployees(ctx httpapi.Context) {
	principal, ok := employeePrincipal(ctx)
	if !ok {
		return
	}
	items, err := a.Service.ListEmployees(ctx.Context(), principal.AccountKey, principal.TenantID)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	if items == nil {
		items = []service.EmployeeView{}
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", items)
}

func (a *AIAPI) CreateEmployee(ctx httpapi.Context) {
	principal, ok := employeePrincipal(ctx)
	if !ok {
		return
	}
	var req service.CreateEmployeeRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式不正确")
		return
	}
	item, err := a.Service.CreateEmployee(ctx.Context(), principal.AccountKey, req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}

func (a *AIAPI) UpdateEmployee(ctx httpapi.Context) {
	principal, ok := employeePrincipal(ctx)
	if !ok {
		return
	}
	var req service.UpdateEmployeeRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式不正确")
		return
	}
	item, err := a.Service.UpdateEmployee(ctx.Context(), principal.AccountKey, principal.TenantID, ctx.Param("id"), req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}

func (a *AIAPI) DeleteEmployee(ctx httpapi.Context) {
	principal, ok := employeePrincipal(ctx)
	if !ok {
		return
	}
	if err := a.Service.DeleteEmployee(ctx.Context(), principal.AccountKey, ctx.Param("id")); err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", map[string]any{"employeeId": ctx.Param("id")})
}

func (a *AIAPI) ListEmployeeExecutions(ctx httpapi.Context) {
	principal, ok := employeePrincipal(ctx)
	if !ok {
		return
	}
	limit := 0
	if raw := strings.TrimSpace(ctx.Query("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "limit 不正确")
			return
		}
		limit = parsed
	}
	items, err := a.Service.ListEmployeeExecutions(ctx.Context(), principal.AccountKey, ctx.Param("id"), limit)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	if items == nil {
		items = []service.EmployeeExecutionView{}
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", items)
}

func (a *AIAPI) ListEmployeeChannels(ctx httpapi.Context) {
	principal, ok := employeePrincipal(ctx)
	if !ok {
		return
	}
	items, err := a.Service.ListEmployeeChannels(ctx.Context(), principal.TenantID)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	if items == nil {
		items = []service.ChannelBinding{}
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", items)
}

func (a *AIAPI) EmployeeRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/ai")
	routes.Get("/employees", a.ListEmployees)
	routes.Get("/employees/:id/executions", a.ListEmployeeExecutions)
	routes.Post("/employees", a.CreateEmployee)
	routes.Patch("/employees/:id", a.UpdateEmployee)
	routes.Delete("/employees/:id", a.DeleteEmployee)
	routes.Get("/employee-channels", a.ListEmployeeChannels)
	return routes
}

func employeePrincipal(ctx httpapi.Context) (iampkg.Principal, bool) {
	principal, ok := iampkg.PrincipalFromContext(ctx.Context())
	if !ok || strings.TrimSpace(principal.AccountKey) == "" {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录")
		return iampkg.Principal{}, false
	}
	return principal, true
}
