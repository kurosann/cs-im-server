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

type Module struct{}

func (m *Module) Init(_ context.Context) error {
	service := relayService.NewRelayService(gatewaypkg.GetRealtimePublisher())
	if err := service.Load(); err != nil {
		return err
	}
	relaypkg.SetService(service)
	return nil
}

func (m *Module) Model() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{
		commonmodule.NewModelRegInfo(&relaymodel.Channel{}),
		commonmodule.NewModelRegInfo(&relaymodel.Webhook{}),
		commonmodule.NewModelRegInfo(&relaymodel.AIConfig{}),
		commonmodule.NewModelRegInfo(&relaymodel.AIExecution{}),
		commonmodule.NewModelRegInfo(&relaymodel.Delivery{}),
	}
}

func (m *Module) Api() []commonmodule.ApiRegInfo {
	return []commonmodule.ApiRegInfo{
		commonmodule.NewApiRegInfo(
			relayv1.RelayChannelService_ServiceDesc,
			relaygrpc.NewRelayChannelGRPCAPI(iampkg.Service(), relaypkg.Service()),
		),
	}
}
func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }

func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	channels := relayhttp.NewChannelAPI(relaypkg.Service())
	webhooks := relayhttp.NewWebhookAPI(relaypkg.Service())
	ai := relayhttp.NewAIAPI(relaypkg.Service())
	realtime := relayhttp.NewRealtimeAPI(gatewaypkg.GetAppAuthorizer(), gatewaypkg.GetMessageIngress(), gatewaypkg.GetRealtimeSubscriber())
	relayMessages := relayhttp.NewRelayMessageAPI(gatewaypkg.GetMessageEgress(), gatewaypkg.GetRealtimeSubscriber())
	admin := commonmodule.NewHttpRouteGroup("/admin/v1").
		Compose(channels.Routes(), webhooks.Routes(), ai.Routes())
	channelEvents := commonmodule.NewHttpRouteGroup("/api/v1").Compose(realtime.Routes())
	relayEvents := relayMessages.Routes()
	return commonmodule.HttpApiInfo{
		Handlers:    commonmodule.MergeHandlers(admin.Handlers(), channelEvents.Handlers(), relayEvents.Handlers()),
		Middlewares: []commonmodule.HttpMiddleware{iamhttp.AccountTokenMiddleware()},
	}
}
