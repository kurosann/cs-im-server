package gateway

import (
	"context"

	"cs-im-server/internal/cfg"
	"cs-im-server/internal/module/gateway/internal/api"
	"cs-im-server/pkg/httpmiddleware"
	commonmodule "vane-common/module"
)

var _ commonmodule.IModule = (*GatewayModule)(nil)
var _ commonmodule.IHTTPModule = (*GatewayModule)(nil)

type GatewayModule struct {
	apps          *api.AppAPI
	customers     *api.CustomerAPI
	conversations *api.ConversationAPI
	messages      *api.MessageAPI
	channels      *api.ChannelAPI
}

func NewGatewayModule() *GatewayModule {
	return &GatewayModule{
		apps:          &api.AppAPI{},
		customers:     &api.CustomerAPI{},
		conversations: &api.ConversationAPI{},
		messages:      &api.MessageAPI{},
		channels:      &api.ChannelAPI{},
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
	return commonmodule.HttpApiInfo{
		Prefix: "/api/v1",
		Middlewares: []commonmodule.HttpMiddleware{
			httpmiddleware.AccessToken(func() string {
				if cfg.GetConfig() == nil {
					return ""
				}
				return cfg.GetConfig().Access.AccessToken
			}),
		},
		Handlers: []commonmodule.HttpHandlerRegInfo{
			commonmodule.NewPostReqHttpRegInfo("/apps/register", m.apps.Register),
			commonmodule.NewGetReqHttpRegInfo("/apps", m.apps.List),
			commonmodule.NewGetReqHttpRegInfo("/apps/:id", m.apps.Get),
			commonmodule.NewPostReqHttpRegInfo("/apps/heartbeat", m.apps.Heartbeat),
			commonmodule.NewPostReqHttpRegInfo("/customers", m.customers.Create),
			commonmodule.NewGetReqHttpRegInfo("/customers/:id", m.customers.Get),
			commonmodule.NewPostReqHttpRegInfo("/conversations", m.conversations.Create),
			commonmodule.NewGetReqHttpRegInfo("/conversations", m.conversations.List),
			commonmodule.NewGetReqHttpRegInfo("/conversations/:id", m.conversations.Get),
			commonmodule.NewPostReqHttpRegInfo("/conversations/:id/assign", m.conversations.Assign),
			commonmodule.NewPostReqHttpRegInfo("/conversations/:id/close", m.conversations.Close),
			commonmodule.NewPostReqHttpRegInfo("/conversations/:id/forward", m.conversations.Forward),
			commonmodule.NewGetReqHttpRegInfo("/conversations/:id/messages", m.conversations.Messages),
			commonmodule.NewGetReqHttpRegInfo("/conversations/:id/deliveries", m.conversations.Deliveries),
			commonmodule.NewPostReqHttpRegInfo("/messages", m.messages.Create),
			commonmodule.NewPostReqHttpRegInfo("/messages/:id/status", m.messages.UpdateStatus),
			commonmodule.NewPostReqHttpRegInfo("/channels", m.channels.Create),
			commonmodule.NewGetReqHttpRegInfo("/channels", m.channels.List),
			commonmodule.NewPostReqHttpRegInfo("/channels/:id/enable", m.channels.Enable),
			commonmodule.NewPostReqHttpRegInfo("/channels/:id/disable", m.channels.Disable),
		},
	}
}
