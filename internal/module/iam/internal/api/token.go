package api

import (
	iamdto "cs-im-server/internal/module/iam/dto"
	"net/http"

	"cs-im-server/internal/module/iam/internal/service"
	pkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/httpapi"

	commonmodule "vane-common/module"
)

type TokenAPI struct {
	Accounts *service.AccountService
}

func NewTokenAPI(accounts *service.AccountService) *TokenAPI {
	return &TokenAPI{Accounts: accounts}
}

func (a *TokenAPI) AdminRoutes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/tokens")
	routes.Post("", a.Issue)
	routes.Get("", a.List)
	routes.Get("/:tokenKey", a.Get)
	routes.Post("/:tokenKey/disable", a.Disable)
	routes.Post("/:tokenKey/enable", a.Enable)
	routes.Post("/:tokenKey/rotate", a.Rotate)
	return routes
}

func (a *TokenAPI) Issue(ctx httpapi.Context) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	var req iamdto.IssueAccountTokenRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	result, err := a.Accounts.Issue(ctx.Context(), principal, req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", result)
}

func (a *TokenAPI) List(ctx httpapi.Context) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	items, err := a.Accounts.List(ctx.Context(), principal)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", items)
}

func (a *TokenAPI) Get(ctx httpapi.Context) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	item, err := a.Accounts.Get(ctx.Context(), principal, ctx.Param("tokenKey"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}

func (a *TokenAPI) Disable(ctx httpapi.Context) {
	a.setStatus(ctx, "disabled")
}

func (a *TokenAPI) Enable(ctx httpapi.Context) {
	a.setStatus(ctx, "active")
}

func (a *TokenAPI) Rotate(ctx httpapi.Context) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	result, err := a.Accounts.Rotate(ctx.Context(), principal, ctx.Param("tokenKey"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", result)
}

func (a *TokenAPI) setStatus(ctx httpapi.Context, status string) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	item, err := a.Accounts.SetStatus(ctx.Context(), principal, ctx.Param("tokenKey"), status)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}
