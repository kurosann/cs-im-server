package iam

import (
	"net/http"
	"strings"

	commonmodule "vane-common/module"
)

func AccountTokenMiddleware() commonmodule.HttpMiddleware {
	return func(next commonmodule.HttpHandler) commonmodule.HttpHandler {
		return func(ctx commonmodule.HttpReqContext) {
			token := strings.TrimPrefix(ctx.Header("Authorization"), "Bearer ")
			if token == "" {
				token = ctx.Header("X-Access-Token")
			}
			principal, err := Service().Authenticate(ctx.Context(), token)
			if err != nil {
				_ = ctx.JSON(http.StatusUnauthorized, map[string]any{
					"code":    "UNAUTHORIZED",
					"message": "invalid account token",
				})
				return
			}
			ctx.SetValue(PrincipalContextKey{}, principal)
			next(ctx)
		}
	}
}

func RequirePrincipalKind(kind string) commonmodule.HttpMiddleware {
	return func(next commonmodule.HttpHandler) commonmodule.HttpHandler {
		return func(ctx commonmodule.HttpReqContext) {
			principal, ok := PrincipalFromContext(ctx.Context())
			if !ok || !principal.HasKind(kind) {
				_ = ctx.JSON(http.StatusForbidden, map[string]any{
					"code":    "FORBIDDEN",
					"message": "principal kind is not allowed",
				})
				return
			}
			next(ctx)
		}
	}
}

func RequirePrincipalScope(scope string) commonmodule.HttpMiddleware {
	return func(next commonmodule.HttpHandler) commonmodule.HttpHandler {
		return func(ctx commonmodule.HttpReqContext) {
			principal, ok := PrincipalFromContext(ctx.Context())
			if !ok || !principal.HasScope(scope) {
				_ = ctx.JSON(http.StatusForbidden, map[string]any{
					"code":    "FORBIDDEN",
					"message": "principal scope is not allowed",
				})
				return
			}
			next(ctx)
		}
	}
}
