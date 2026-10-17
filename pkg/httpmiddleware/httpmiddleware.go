package httpmiddleware

import (
	"net/http"
	"strings"

	"vane-common/module"
)

// AccessToken returns middleware for a caller-provided token source. The
// provider is invoked per request so configuration reloads stay effective.
func AccessToken(provider func() string) module.HttpMiddleware {
	return AccessTokenExcept(provider)
}

func AccessTokenExcept(provider func() string, excludedPrefixes ...string) module.HttpMiddleware {
	return func(next module.HttpHandler) module.HttpHandler {
		return func(ctx module.HttpReqContext) {
			requestPath := ""
			if ctx.Request() != nil && ctx.Request().URL != nil {
				requestPath = ctx.Request().URL.Path
			}
			for _, prefix := range excludedPrefixes {
				if prefix != "" && strings.HasPrefix(requestPath, prefix) {
					next(ctx)
					return
				}
			}
			token := strings.TrimPrefix(ctx.Header("Authorization"), "Bearer ")
			if token == "" {
				token = ctx.Header("X-Access-Token")
			}
			if token == "" || token != provider() {
				_ = ctx.JSON(http.StatusUnauthorized, map[string]any{
					"code":    "UNAUTHORIZED",
					"message": "invalid access token",
				})
				return
			}
			next(ctx)
		}
	}
}
