package gateway

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGatewayHttpApiRoutesRegister(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	for _, route := range (&Module{}).HttpApi().Handlers {
		if route.Handler == nil {
			continue
		}
		engine.Handle(route.Method, route.Path, func(*gin.Context) {})
	}
}
