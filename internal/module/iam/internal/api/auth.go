package api

import (
	"net/http"

	iamdto "cs-im-server/internal/module/iam/dto"
	"cs-im-server/internal/module/iam/internal/service"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	"cs-im-server/pkg/httpapi"
)

type AuthAPI struct {
	Accounts *service.AccountService
	Staff    *service.StaffService
}

func NewAuthAPI(accounts *service.AccountService, staff *service.StaffService) *AuthAPI {
	return &AuthAPI{Accounts: accounts, Staff: staff}
}

type RegisterRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	TenantName  string `json:"tenantName"`
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
	principal, _ := iamprincipal.FromContext(ctx.Context())
	if err := a.Staff.Logout(ctx.Context(), principal); err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", nil)
}

func (a *AuthAPI) ChangePassword(ctx httpapi.Context) {
	principal, _ := iamprincipal.FromContext(ctx.Context())
	var req iamdto.ChangeOwnPasswordRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	if err := a.Staff.ChangeOwnPassword(ctx.Context(), principal, req); err != nil {
		writeServiceError(ctx, err)
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", "password_changed")
}

func (a *AuthAPI) Me(ctx httpapi.Context) {
	principal, _ := iamprincipal.FromContext(ctx.Context())
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", map[string]any{
		"userKey":  principal.AccountKey,
		"tenantId": principal.TenantID,
		"kind":     principal.Kind,
		"scopes":   principal.Scopes,
	})
}
