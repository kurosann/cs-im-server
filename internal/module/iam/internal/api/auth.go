package api

import (
	"net/http"
	"strings"

	"cs-im-server/pkg/httpapi"

	commonmodule "vane-common/module"
)

const (
	demoAccount  = "demo"
	demoEmail    = "demo@cs-im.local"
	demoPassword = "demo1234"
)

type AuthAPI struct {
	AccessToken string
}

func NewAuthAPI(accessToken string) *AuthAPI {
	if strings.TrimSpace(accessToken) == "" {
		accessToken = "dev-access-token"
	}
	return &AuthAPI{AccessToken: accessToken}
}

func (a *AuthAPI) Routes() commonmodule.HttpRouteGroup {
	routes := commonmodule.NewHttpRouteGroup("")
	routes.Post("/login", a.Login)
	routes.Post("/logout", a.Logout)
	return routes
}

type loginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

func (a *AuthAPI) Login(ctx httpapi.Context) {
	var req loginRequest
	if err := ctx.BindJSON(&req); err != nil {
		httpapi.WriteError(ctx, http.StatusBadRequest, "BAD_REQUEST", "请求格式不正确")
		return
	}
	identifier := strings.ToLower(strings.TrimSpace(req.Identifier))
	if (identifier != demoAccount && identifier != demoEmail) || req.Password != demoPassword {
		httpapi.WriteError(ctx, http.StatusUnauthorized, "UNAUTHORIZED", "账号或密码错误")
		return
	}
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", map[string]any{
		"user": map[string]any{
			"userKey":     demoAccount,
			"account":     demoAccount,
			"displayName": "演示账号",
			"email":       demoEmail,
			"role":        "admin",
		},
		"token": map[string]any{
			"token": a.AccessToken,
		},
	})
}

func (a *AuthAPI) Logout(ctx httpapi.Context) {
	httpapi.WriteJSON(ctx, http.StatusOK, "OK", map[string]any{})
}
