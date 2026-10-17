package channel

import (
	"testing"

	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	"github.com/gin-gonic/gin"
)

func TestChannelHttpApiRoutesRegister(t *testing.T) {
	gatewaypkg.NewTestGatewayServices()

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	for _, route := range (&Module{}).HttpApi().Handlers {
		if route.Handler == nil {
			continue
		}
		engine.Handle(route.Method, route.Path, func(*gin.Context) {})
	}
}
