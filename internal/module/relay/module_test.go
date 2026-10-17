package relay

import (
	"testing"

	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	relayService "cs-im-server/internal/module/relay/internal/service"
	relaypkg "cs-im-server/internal/module/relay/pkg"
	"github.com/gin-gonic/gin"
)

func TestRelayHttpApiRoutesRegister(t *testing.T) {
	realtime := gatewaypkg.NewRealtimeService()
	apps := gatewaypkg.NewAppService(realtime)
	customers := gatewaypkg.NewCustomerService(realtime)
	conversations := gatewaypkg.NewConversationService(apps, customers, nil, realtime)
	messages := gatewaypkg.NewMessageService(apps, conversations, realtime)
	conversations.SetMessages(gatewaypkg.NewConversationMessageStore(messages))
	gatewaypkg.SetServices(realtime, apps, customers, conversations, messages)
	relaypkg.SetService(relayService.NewRelayService(realtime))

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	for _, route := range (&Module{}).HttpApi().Handlers {
		if route.Handler == nil {
			continue
		}
		engine.Handle(route.Method, route.Path, func(*gin.Context) {})
	}
}
