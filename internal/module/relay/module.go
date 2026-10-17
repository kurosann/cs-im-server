package relay

import (
	"context"

	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg"
	relayhttp "cs-im-server/internal/module/relay/internal/http"
	relaymodel "cs-im-server/internal/module/relay/internal/model"
	relayService "cs-im-server/internal/module/relay/internal/service"
	relaypkg "cs-im-server/internal/module/relay/pkg"
	commonmodule "vane-common/module"
)

var _ commonmodule.IModule = (*Module)(nil)
var _ commonmodule.IHTTPModule = (*Module)(nil)
var _ commonmodule.IModuleDb = (*Module)(nil)

type Module struct{}

func (m *Module) Init(_ context.Context) error {
	service := relayService.NewRelayService(gatewaypkg.GetRealtime())
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

func (m *Module) Api() []commonmodule.ApiRegInfo   { return nil }
func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }

func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	channels := relayhttp.NewChannelAPI(relaypkg.Service())
	webhooks := relayhttp.NewWebhookAPI(relaypkg.Service())
	ai := relayhttp.NewAIAPI(relaypkg.Service())
	admin := commonmodule.NewHttpRouteGroup("/admin/v1").
		Compose(channels.Routes(), webhooks.Routes(), ai.Routes())
	return commonmodule.HttpApiInfo{
		Handlers:    admin.Handlers(),
		Middlewares: []commonmodule.HttpMiddleware{iampkg.AccountTokenMiddleware()},
	}
}
