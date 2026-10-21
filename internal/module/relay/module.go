package relay

import (
	"context"
	iamhttp "cs-im-server/internal/module/iam/pkg/http"

	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	relaygrpc "cs-im-server/internal/module/relay/internal/grpc"
	relayhttp "cs-im-server/internal/module/relay/internal/http"
	relaymodel "cs-im-server/internal/module/relay/internal/model"
	relayService "cs-im-server/internal/module/relay/internal/service"
	relaypkg "cs-im-server/internal/module/relay/pkg"
	relayv1 "github.com/kurosann/cs-im-internal-sdk/relay/v1"
	commonmodule "vane-common/module"
)

var _ commonmodule.IModule = (*Module)(nil)
var _ commonmodule.IHTTPModule = (*Module)(nil)
var _ commonmodule.IModuleDb = (*Module)(nil)

type Module struct {
	accounts *iampkg.AccountService
	relay    *relayService.RelayService
}

func (m *Module) Init(_ context.Context) error {
	m.accounts = iampkg.Service()
	m.relay = relayService.NewRelayService(gatewaypkg.GetRealtimePublisher())
	if err := m.relay.Load(); err != nil {
		return err
	}
	relaypkg.SetService(m.relay)
	return nil
}

func (m *Module) Model() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{
		commonmodule.NewModelRegInfo(&relaymodel.Channel{}),
		commonmodule.NewModelRegInfo(&relaymodel.Webhook{}),
		commonmodule.NewModelRegInfo(&relaymodel.AIConfig{}),
		commonmodule.NewModelRegInfo(&relaymodel.AIExecution{}),
		commonmodule.NewModelRegInfo(&relaymodel.AIEmployee{}),
		commonmodule.NewModelRegInfo(&relaymodel.AIEmployeeBinding{}),
		commonmodule.NewModelRegInfo(&relaymodel.AIEmployeeExecution{}),
		commonmodule.NewModelRegInfo(&relaymodel.Delivery{}),
	}
}

func (m *Module) Api() []commonmodule.ApiRegInfo {
	return []commonmodule.ApiRegInfo{
		commonmodule.NewApiRegInfo(
			relayv1.RelayChannelService_ServiceDesc,
			relaygrpc.NewRelayChannelGRPCAPI(m.accounts, m.relay),
		),
		commonmodule.NewApiRegInfo(
			relayv1.RelayAccessService_ServiceDesc,
			relaygrpc.NewRelayAccessGRPCAPI(
				m.accounts,
				gatewaypkg.GetMessageEgress(),
				gatewaypkg.GetRealtimeSubscriber(),
			),
		),
		commonmodule.NewApiRegInfo(
			relayv1.RelayEmployeeService_ServiceDesc,
			relaygrpc.NewRelayEmployeeGRPCAPI(m.accounts, m.relay),
		),
		commonmodule.NewApiRegInfo(
			relayv1.RelayWebhookService_ServiceDesc,
			relaygrpc.NewRelayWebhookGRPCAPI(m.accounts, m.relay),
		),
		commonmodule.NewApiRegInfo(
			relayv1.RelayAIService_ServiceDesc,
			relaygrpc.NewRelayAIGRPCAPI(m.accounts, m.relay),
		),
	}
}
func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }

func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	channels := relayhttp.NewChannelAPI(m.relay)
	webhooks := relayhttp.NewWebhookAPI(m.relay)
	ai := relayhttp.NewAIAPI(m.relay)
	realtime := relayhttp.NewRealtimeAPI(gatewaypkg.GetAppAuthorizer(), gatewaypkg.GetMessageIngress(), gatewaypkg.GetRealtimeSubscriber())
	relayMessages := relayhttp.NewRelayMessageAPI(gatewaypkg.GetMessageEgress(), gatewaypkg.GetRealtimeSubscriber())
	admin := commonmodule.NewHttpRouteGroup("/admin/v1").
		Compose(channels.Routes(), webhooks.Routes(), ai.Routes())
	channelEvents := commonmodule.NewHttpRouteGroup("/api/v1").Compose(realtime.Routes(), ai.EmployeeRoutes())
	relayEvents := relayMessages.Routes()
	return commonmodule.HttpApiInfo{
		Handlers:    commonmodule.MergeHandlers(admin.Handlers(), channelEvents.Handlers(), relayEvents.Handlers()),
		Middlewares: []commonmodule.HttpMiddleware{iamhttp.AccountTokenMiddleware(m.accounts)},
	}
}
