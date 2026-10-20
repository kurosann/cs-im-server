package api

import (
	"net/http"

	iamdto "cs-im-server/internal/module/iam/dto"
	"cs-im-server/internal/module/iam/internal/service"
	iamhttp "cs-im-server/internal/module/iam/pkg/http"
	pkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/httpapi"

	commonmodule "vane-common/module"
)

type AuthAPI struct {
	Staff *service.StaffService
}

func NewAuthAPI(staff *service.StaffService) *AuthAPI {
	return &AuthAPI{Staff: staff}
}

type RegisterRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	TenantName  string `json:"tenantName"`
}

func (a *AuthAPI) Routes() []commonmodule.HttpHandlerRegInfo {
	auth := commonmodule.NewHttpRouteGroup("/api/v1/auth")
	return []commonmodule.HttpHandlerRegInfo{
		auth.Post("/login", a.Login),
		auth.Post("/register", a.Register),
		auth.Post("/logout", a.Logout, commonmodule.WithMiddlewares(iamhttp.AccountTokenMiddleware(), iamhttp.RequirePrincipalKind(pkg.PrincipalKindHuman))),
		auth.Get("/me", a.Me, commonmodule.WithMiddlewares(iamhttp.AccountTokenMiddleware(), iamhttp.RequirePrincipalKind(pkg.PrincipalKindHuman))),
	}
}

func (a *AuthAPI) Register(ctx httpapi.Context) {
	var req RegisterRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	result, err := a.Staff.Register(ctx.Context(), iamdto.RegisterRequest(req))
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", result)
}

func (a *AuthAPI) Login(ctx httpapi.Context) {
	var req iamdto.LoginRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	requestContext := service.WithAuditContext(ctx.Context(), iamdto.AuditContext{
		Device: ctx.Header("User-Agent"),
		IP:     ctx.ClientIP(),
	})
	result, err := a.Staff.Login(requestContext, req, iamdto.SessionContext{
		Device: ctx.Header("User-Agent"),
		IP:     ctx.ClientIP(),
	})
	if err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", result)
}

func (a *AuthAPI) Logout(ctx httpapi.Context) {
	principal, _ := pkg.PrincipalFromContext(ctx.Context())
	if err := a.Staff.Logout(ctx.Context(), principal); err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", nil)
}

func (a *AuthAPI) Me(ctx httpapi.Context) {
	principal, _ := pkg.PrincipalFromContext(ctx.Context())
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", map[string]any{
		"userKey":  principal.AccountKey,
		"tenantId": principal.TenantID,
		"kind":     principal.Kind,
		"scopes":   principal.Scopes,
	})
}
