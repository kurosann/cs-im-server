package channel

import (
	"testing"

	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	"cs-im-server/internal/module/gateway/service"
	"github.com/gin-gonic/gin"
)

func TestChannelHttpApiRoutesRegister(t *testing.T) {
	realtime := gatewaypkg.NewRealtimeService()
	apps := gatewaypkg.NewAppService(realtime)
	customers := gatewaypkg.NewCustomerService(realtime)
	conversations := gatewaypkg.NewConversationService(apps, customers, nil, realtime)
	messages := gatewaypkg.NewMessageService(apps, conversations, realtime)
	conversations.SetMessages(service.NewConversationMessageStore(messages))
	gatewaypkg.SetServices(realtime, apps, customers, conversations, messages)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	for _, route := range (&Module{}).HttpApi().Handlers {
		if route.Handler == nil {
			continue
		}
		engine.Handle(route.Method, route.Path, func(*gin.Context) {})
	}
}
