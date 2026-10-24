package http

import (
	nethttp "net/http"
	"strings"

	iamdto "cs-im-server/internal/module/iam/dto"
	service "cs-im-server/internal/module/iam/internal/service"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	commonmodule "vane-common/module"
)

func AccountTokenMiddleware(accounts *service.AccountService) commonmodule.HttpMiddleware {
	return func(next commonmodule.HttpHandler) commonmodule.HttpHandler {
		return func(ctx commonmodule.HttpReqContext) {
			token := strings.TrimPrefix(ctx.Header("Authorization"), "Bearer ")
			if token == "" {
				token = ctx.Header("X-Access-Token")
			}
			principal, err := accounts.Authenticate(ctx.Context(), token)
			if err != nil {
				_ = ctx.JSON(nethttp.StatusUnauthorized, map[string]any{
					"code":    "UNAUTHORIZED",
					"message": "invalid account token",
				})
				return
			}
			ctx.SetValue(iamprincipal.ContextKey{}, principal)
			ctx.SetValue(service.AuditContextKey{}, iamdto.AuditContext{
				Device: ctx.Header("User-Agent"),
				IP:     ctx.ClientIP(),
			})
			next(ctx)
		}
	}
}

func RequirePrincipalKind(kind string) commonmodule.HttpMiddleware {
	return func(next commonmodule.HttpHandler) commonmodule.HttpHandler {
		return func(ctx commonmodule.HttpReqContext) {
			principal, ok := iamprincipal.FromContext(ctx.Context())
			if !ok || !principal.HasKind(kind) {
				_ = ctx.JSON(nethttp.StatusForbidden, map[string]any{
					"code":    "FORBIDDEN",
					"message": "principal kind is not allowed",
				})
				return
			}
			next(ctx)
		}
	}
}

func RequirePrincipalAnyKind(kinds ...string) commonmodule.HttpMiddleware {
	return func(next commonmodule.HttpHandler) commonmodule.HttpHandler {
		return func(ctx commonmodule.HttpReqContext) {
			principal, ok := iamprincipal.FromContext(ctx.Context())
			if !ok || !anyKindAllowed(principal, kinds...) {
				_ = ctx.JSON(nethttp.StatusForbidden, map[string]any{
					"code":    "FORBIDDEN",
					"message": "principal kind is not allowed",
				})
				return
			}
			next(ctx)
		}
	}
}

func anyKindAllowed(principal iamprincipal.Principal, kinds ...string) bool {
	for _, kind := range kinds {
		if principal.HasKind(kind) {
			return true
		}
	}
	return false
}

func RequirePrincipalScope(scope string) commonmodule.HttpMiddleware {
	return func(next commonmodule.HttpHandler) commonmodule.HttpHandler {
		return func(ctx commonmodule.HttpReqContext) {
			principal, ok := iamprincipal.FromContext(ctx.Context())
			if !ok || !principal.HasScope(scope) {
				_ = ctx.JSON(nethttp.StatusForbidden, map[string]any{
					"code":    "FORBIDDEN",
					"message": "principal scope is not allowed",
				})
				return
			}
			next(ctx)
		}
	}
}
