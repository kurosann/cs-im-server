package gateway

import (
	"context"

	"cs-im-server/internal/module/gateway/api"
	"cs-im-server/internal/module/gateway/model"
	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	"cs-im-server/internal/module/gateway/service"
	iampkg "cs-im-server/internal/module/iam/pkg"

	commonmodule "vane-common/module"
)

var _ commonmodule.IModule = (*Module)(nil)
var _ commonmodule.IHTTPModule = (*Module)(nil)
var _ commonmodule.IModuleDb = (*Module)(nil)

type Module struct {
	realtime      *service.RealtimeService
	apps          *service.AppService
	customers     *service.CustomerService
	conversations *service.ConversationService
	messages      *service.MessageService
}

func (m *Module) Init(_ context.Context) error {
	m.realtime = service.NewRealtimeService()
	m.apps = service.NewAppService(m.realtime)
	m.customers = service.NewCustomerService(m.realtime)
	m.conversations = service.NewConversationService(m.apps, m.customers, nil, m.realtime)
	m.messages = service.NewMessageService(m.apps, m.conversations, m.realtime)
	m.conversations.SetMessages(service.NewConversationMessageStore(m.messages))
	gatewaypkg.SetServices(m.realtime, m.apps, m.customers, m.conversations, m.messages)

	for _, loaded := range []interface{ Load() error }{m.apps, m.customers, m.conversations, m.messages} {
		if err := loaded.Load(); err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) Model() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{
		commonmodule.NewModelRegInfo(&model.App{}),
		commonmodule.NewModelRegInfo(&model.Customer{}),
		commonmodule.NewModelRegInfo(&model.Conversation{}),
		commonmodule.NewModelRegInfo(&model.Message{}),
	}
}

func (m *Module) Api() []commonmodule.ApiRegInfo   { return nil }
func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }

func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	appAPI := api.NewAppAPI(m.apps, m.messages)
	customerAPI := api.NewCustomerAPI(m.customers)
	conversationAPI := api.NewConversationAPI(m.conversations)
	messageAPI := api.NewMessageAPI(m.messages)

	adminGroup := commonmodule.NewHttpRouteGroup("/admin/v1").Compose(
		appAPI.AdminRoutes(),
		customerAPI.AdminRoutes(),
		conversationAPI.AdminRoutes(),
		messageAPI.AdminRoutes(),
		messageAPI.AdminConversationRoutes(),
	)

	return commonmodule.HttpApiInfo{
		Handlers:    adminGroup.Handlers(),
		Middlewares: []commonmodule.HttpMiddleware{iampkg.AccountTokenMiddleware()},
	}
}
