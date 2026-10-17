package gateway

import (
	"context"

	"cs-im-server/internal/cfg"
	"cs-im-server/internal/module/gateway/internal/api"
	"cs-im-server/pkg/httpmiddleware"
	commonmodule "vane-common/module"
)

var (
	_ commonmodule.IModule     = (*GatewayModule)(nil)
	_ commonmodule.IHTTPModule = (*GatewayModule)(nil)
)

type GatewayModule struct {
	apps          *api.AppAPI
	customers     *api.CustomerAPI
	conversations *api.ConversationAPI
	messages      *api.MessageAPI
	channels      *api.ChannelAPI
	ai            *api.AIAPI
	webhooks      *api.WebhookAPI
	realtime      *api.RealtimeAPI
}

func NewGatewayModule() *GatewayModule {
	return &GatewayModule{
		apps:          &api.AppAPI{},
		customers:     &api.CustomerAPI{},
		conversations: &api.ConversationAPI{},
		messages:      &api.MessageAPI{},
		channels:      &api.ChannelAPI{},
		ai:            &api.AIAPI{},
		webhooks:      &api.WebhookAPI{},
		realtime:      &api.RealtimeAPI{},
	}
}

func (m *GatewayModule) Init(_ context.Context) error {
	return nil
}

func (m *GatewayModule) Api() []commonmodule.ApiRegInfo {
	return []commonmodule.ApiRegInfo{}
}

func (m *GatewayModule) Job() []commonmodule.JobRegInfo {
	return []commonmodule.JobRegInfo{}
}

func (m *GatewayModule) Task() []commonmodule.TaskRegInfo {
	return []commonmodule.TaskRegInfo{}
}

func (m *GatewayModule) HttpApi() commonmodule.HttpApiInfo {
	accessToken := httpmiddleware.AccessToken(func() string {
		if cfg.GetConfig() == nil {
			return ""
		}
		return cfg.GetConfig().Access.AccessToken
	})
	apiGroup := commonmodule.NewHttpRouteGroup("/api/v1", commonmodule.WithGroupMiddlewares(accessToken)).
		Compose(
			m.apps.Routes(),
			m.customers.Routes(),
			m.conversations.Routes(),
			m.messages.Routes(),
			m.channels.Routes(),
			m.ai.Routes(),
			m.webhooks.Routes(),
		)
	realtimeGroup := commonmodule.NewHttpRouteGroup("/ws/v1", commonmodule.WithGroupMiddlewares(accessToken)).
		Compose(m.realtime.Routes())

	return commonmodule.HttpApiInfo{
		Prefix:   "",
		Handlers: commonmodule.MergeHandlers(apiGroup.Handlers(), realtimeGroup.Handlers()),
	}
}
