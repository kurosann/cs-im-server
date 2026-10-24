package gateway

import (
	"context"
	iamhttp "cs-im-server/internal/module/iam/pkg/http"

	"cs-im-server/internal/cfg"
	"cs-im-server/internal/module/gateway/internal/api"
	"cs-im-server/internal/module/gateway/internal/model"
	gatewayruntime "cs-im-server/internal/module/gateway/internal/runtime"
	"cs-im-server/internal/module/gateway/internal/service"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"

	commonmodule "vane-common/module"
)

var _ commonmodule.IModule = (*Module)(nil)
var _ commonmodule.IHTTPModule = (*Module)(nil)
var _ commonmodule.IModuleDb = (*Module)(nil)

type Module struct {
	accounts      *iampkg.AccountService
	realtime      *service.RealtimeService
	apps          *service.AppService
	customers     *service.CustomerService
	conversations *service.ConversationService
	messages      *service.MessageService
}

func (m *Module) Init(_ context.Context) error {
	m.accounts = iampkg.Service()
	var realtimeBus service.RealtimeBus
	if !cfg.GetServerConfig().Standalone {
		realtimeBus = service.NewPublishServerRealtimeBus()
	}
	m.realtime = service.NewRealtimeService(realtimeBus)
	m.apps = service.NewAppService(m.realtime)
	m.customers = service.NewCustomerService(m.realtime)
	messageActivity := service.NewMessageActivity(m.realtime)
	m.conversations = service.NewConversationService(m.apps, m.customers, m.realtime, messageActivity)
	m.messages = service.NewMessageService(m.apps, m.conversations, m.realtime)
	gatewayruntime.SetCapabilities(m.apps, m.conversations, m.messages, m.realtime, m.realtime, m.messages)

	return nil
}

func (m *Module) Model() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{
		commonmodule.NewModelRegInfo(&model.App{}),
		commonmodule.NewModelRegInfo(&model.Customer{}),
		commonmodule.NewModelRegInfo(&model.Conversation{}),
		commonmodule.NewModelRegInfo(&model.Message{}),
		commonmodule.NewModelRegInfo(&model.HistoryRequest{}),
	}
}

func (m *Module) Api() []commonmodule.ApiRegInfo   { return nil }
func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }

func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	appAPI := api.NewAppAPI(m.apps, m.messages)
	customerAPI := api.NewCustomerAPI(m.customers, m.conversations)
	conversationAPI := api.NewConversationAPI(m.conversations)
	messageAPI := api.NewMessageAPI(m.messages)
	realtimeAPI := api.NewRealtimeAPI(m.realtime, m.conversations)
	monitorAPI := api.NewMonitorAPI(m.apps, m.conversations)

	adminGroup := commonmodule.NewHttpRouteGroup("/admin/v1").Compose(
		appAPI.AdminRoutes(),
		customerAPI.AdminRoutes(),
		conversationAPI.AdminRoutes(),
		messageAPI.AdminRoutes(),
		messageAPI.AdminConversationRoutes(),
		realtimeAPI.AdminRoutes(),
		monitorAPI.AdminRoutes(),
	)

	return commonmodule.HttpApiInfo{
		Handlers: adminGroup.Handlers(),
		Middlewares: []commonmodule.HttpMiddleware{
			iamhttp.AccountTokenMiddleware(m.accounts),
			iamhttp.RequirePrincipalKind(iampkg.PrincipalKindHuman),
			iamhttp.RequirePrincipalScope(iampkg.ScopeGatewayAdmin),
		},
	}
}
