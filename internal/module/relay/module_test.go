package relay

import (
	"testing"

	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	relayService "cs-im-server/internal/module/relay/internal/service"
	relaypkg "cs-im-server/internal/module/relay/pkg"
	"github.com/gin-gonic/gin"
)

func TestRelayHttpApiRoutesRegister(t *testing.T) {
	services := gatewaypkg.NewTestGatewayServices()
	relaypkg.SetService(relayService.NewRelayService(services.Realtime))

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	for _, route := range (&Module{}).HttpApi().Handlers {
		if route.Handler == nil {
			continue
		}
		engine.Handle(route.Method, route.Path, func(*gin.Context) {})
	}
}
