package relay

import (
	"context"
	iamhttp "cs-im-server/internal/module/iam/pkg/http"

	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/internal/module/relay/internal/gatewaybridge"
	relaygrpc "cs-im-server/internal/module/relay/internal/grpc"
	relayhttp "cs-im-server/internal/module/relay/internal/http"
	relaymodel "cs-im-server/internal/module/relay/internal/model"
	relayService "cs-im-server/internal/module/relay/internal/service"
	translationpkg "cs-im-server/internal/module/translation/pkg/translation"
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
	m.relay = relayService.NewRelayService(relayService.RuntimeDeps{
		Realtime: gatewaypkg.GetRealtimePublisher(),
		Inbound:  gatewaybridge.NewGatewayBridge(),
	})
	relayService.SetGlobalService(m.relay)
	gatewaybridge.ObserveCustomerMessages(m.relay)
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
		commonmodule.NewModelRegInfo(&relaymodel.KnowledgeGroup{}),
		commonmodule.NewModelRegInfo(&relaymodel.KnowledgeBase{}),
		commonmodule.NewModelRegInfo(&relaymodel.KnowledgeDoc{}),
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
	knowledge := relayhttp.NewKnowledgeAPI(m.relay)
	relayMessages := relayhttp.NewRelayMessageAPI(gatewaypkg.GetMessageEgress(), gatewaypkg.GetRealtimeSubscriber())
	relayTranslations := relayhttp.NewRelayTranslationAPI(translationpkg.Service)
	admin := commonmodule.NewHttpRouteGroup("/admin/v1", commonmodule.WithGroupMiddlewares(
		iamhttp.RequirePrincipalKind(iampkg.PrincipalKindHuman),
		iamhttp.RequirePrincipalScope(iampkg.ScopeGatewayAdmin),
	)).Compose(channels.Routes(), webhooks.Routes(), ai.Routes(), ai.EmployeeRoutes(), knowledge.Routes())
	relayEvents := commonmodule.NewHttpRouteGroup("/api/v1/relays", commonmodule.WithGroupMiddlewares(
		iamhttp.RequirePrincipalAnyKind(iampkg.PrincipalKindRelay, iampkg.PrincipalKindHuman),
		iamhttp.RequirePrincipalScope(iampkg.ScopeRelayAccess),
	)).Compose(relayMessages.Routes(), relayTranslations.Routes())
	return commonmodule.HttpApiInfo{
		Handlers:    commonmodule.MergeHandlers(admin.Handlers(), relayEvents.Handlers()),
		Middlewares: []commonmodule.HttpMiddleware{iamhttp.AccountTokenMiddleware(m.accounts)},
	}
}
