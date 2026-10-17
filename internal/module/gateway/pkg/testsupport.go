package gateway

import (
	"context"

	gatewayruntime "cs-im-server/internal/module/gateway/internal/runtime"
	"cs-im-server/internal/module/gateway/internal/service"
	iampkg "cs-im-server/internal/module/iam/pkg"
)

type AppTestService interface {
	Register(ctx context.Context, principal iampkg.Principal, req RegisterAppRequest) (App, error)
	AppAuthorizer
}

type RealtimeTestService interface {
	RealtimeSubscriber
	RealtimePublisher
}

type GatewayTestServices struct {
	Apps          AppTestService
	Conversations ConversationIngress
	Messages      MessageIngress
	Realtime      RealtimeTestService
}

func NewTestGatewayServices() GatewayTestServices {
	realtime := service.NewRealtimeService()
	apps := service.NewAppService(realtime)
	customers := service.NewCustomerService(realtime)
	conversations := service.NewConversationService(apps, customers, nil, realtime)
	messages := service.NewMessageService(apps, conversations, realtime)
	conversations.SetMessages(service.NewConversationMessageStore(messages))

	services := GatewayTestServices{
		Apps: apps, Conversations: conversations,
		Messages: messages, Realtime: realtime,
	}
	gatewayruntime.SetCapabilities(apps, conversations, messages, realtime, realtime)
	return services
}
