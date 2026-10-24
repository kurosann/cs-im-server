package api

import (
	"net/http"

	"cs-im-server/internal/module/iam/internal/service"
	iamhttp "cs-im-server/internal/module/iam/pkg/http"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	"cs-im-server/pkg/httpapi"

	commonmodule "vane-common/module"
)

type RelayLoginRequest struct {
	Token string `json:"token"`
}

type ForwardingSession struct {
	AccountKey string   `json:"accountKey"`
	Name       string   `json:"name"`
	TenantID   string   `json:"tenantId"`
	Kind       string   `json:"kind"`
	Scopes     []string `json:"scopes"`
	Token      string   `json:"token"`
}

func (a *AuthAPI) ManagementRoutes() []commonmodule.HttpHandlerRegInfo {
	auth := commonmodule.NewHttpRouteGroup("/auth/v1/management")
	return []commonmodule.HttpHandlerRegInfo{
		auth.Post("/login", a.Login),
		auth.Post("/register", a.Register),
		auth.Post("/password", a.ChangePassword, commonmodule.WithMiddlewares(
			iamhttp.AccountTokenMiddleware(a.Accounts),
			iamhttp.RequirePrincipalKind(iamprincipal.KindHuman),
		)),
		auth.Post("/logout", a.Logout, commonmodule.WithMiddlewares(
			iamhttp.AccountTokenMiddleware(a.Accounts),
			iamhttp.RequirePrincipalKind(iamprincipal.KindHuman),
		)),
		auth.Get("/me", a.Me, commonmodule.WithMiddlewares(
			iamhttp.AccountTokenMiddleware(a.Accounts),
			iamhttp.RequirePrincipalKind(iamprincipal.KindHuman),
		)),
	}
}

func (a *AuthAPI) ForwardingRoutes() []commonmodule.HttpHandlerRegInfo {
	auth := commonmodule.NewHttpRouteGroup("/auth/v1/forwarding")
	return []commonmodule.HttpHandlerRegInfo{
		auth.Post("/login", a.ForwardingLogin),
		auth.Get("/me", a.ForwardingMe, commonmodule.WithMiddlewares(
			iamhttp.AccountTokenMiddleware(a.Accounts),
			iamhttp.RequirePrincipalKind(iamprincipal.KindRelay),
			iamhttp.RequirePrincipalScope(service.ScopeRelayAccess),
		)),
	}
}

func (a *AuthAPI) ForwardingLogin(ctx httpapi.Context) {
	var req RelayLoginRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	result, err := a.Accounts.IssueForwardingSession(ctx.Context(), req.Token)
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", ForwardingSession{
		AccountKey: result.Token.AccountKey,
		Name:       result.Token.Name,
		TenantID:   result.Token.TenantID,
		Kind:       result.Token.Kind,
		Scopes:     result.Token.Scopes,
		Token:      result.Secret,
	})
}

func (a *AuthAPI) ForwardingMe(ctx httpapi.Context) {
	principal, _ := iamprincipal.FromContext(ctx.Context())
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", ForwardingSession{
		AccountKey: principal.AccountKey,
		Name:       principal.Name,
		TenantID:   principal.TenantID,
		Kind:       principal.Kind,
		Scopes:     principal.Scopes,
	})
}
