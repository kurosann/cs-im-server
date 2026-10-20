package api

import (
	iamdto "cs-im-server/internal/module/iam/dto"
	"net/http"

	"cs-im-server/internal/module/iam/internal/service"
	pkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/httpapi"

	commonmodule "vane-common/module"
)

type ChannelCredentialAPI struct {
	Accounts *service.AccountService
}

func NewChannelCredentialAPI(accounts *service.AccountService) *ChannelCredentialAPI {
	return &ChannelCredentialAPI{Accounts: accounts}
}

func (a *ChannelCredentialAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("/channel-access/apps")
	routes.Post("", a.Issue)
	routes.Get("", a.List)
	routes.Post("/:appId/enable", a.Enable)
	routes.Post("/:appId/disable", a.Disable)
	routes.Post("/:appId/revoke", a.Revoke)
	routes.Post("/:appId/rotate", a.Rotate)
	return routes
}

func (a *ChannelCredentialAPI) Issue(ctx httpapi.Context) {
	var req iamdto.IssueChannelCredentialRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	result, err := a.Accounts.IssueChannelCredential(ctx.Context(), principal, req)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", result)
}

func (a *ChannelCredentialAPI) List(ctx httpapi.Context) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	items, err := a.Accounts.ListChannelCredentials(ctx.Context(), principal)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", items)
}

func (a *ChannelCredentialAPI) Enable(ctx httpapi.Context) {
	a.setStatus(ctx, "active")
}

func (a *ChannelCredentialAPI) Disable(ctx httpapi.Context) {
	a.setStatus(ctx, "disabled")
}

func (a *ChannelCredentialAPI) Revoke(ctx httpapi.Context) {
	a.setStatus(ctx, "revoked")
}

func (a *ChannelCredentialAPI) Rotate(ctx httpapi.Context) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	result, err := a.Accounts.RotateChannelCredential(ctx.Context(), principal, ctx.Param("appId"))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", result)
}

func (a *ChannelCredentialAPI) setStatus(ctx httpapi.Context, status string) {
	principal, ok := pkg.PrincipalFromContext(ctx.Context())
	if !ok {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", pkg.ErrPrincipalMissing.Error())
		return
	}
	item, err := a.Accounts.SetChannelCredentialStatus(ctx.Context(), principal, ctx.Param("appId"), status)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", item)
}
