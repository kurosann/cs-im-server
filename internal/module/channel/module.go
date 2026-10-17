package channel

import (
	"context"

	"cs-im-server/internal/module/channel/internal/grpc"
	"cs-im-server/internal/module/channel/internal/http"
	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg"
	gatewayv1 "github.com/kurosann/cs-im-sdk/gateway/v1"
	commonmodule "vane-common/module"
)

var _ commonmodule.IModule = (*Module)(nil)
var _ commonmodule.IHTTPModule = (*Module)(nil)

type Module struct{}

func (m *Module) Init(_ context.Context) error { return nil }

func (m *Module) Model() []commonmodule.ModelRegInfo { return nil }

func (m *Module) Api() []commonmodule.ApiRegInfo {
	channelGateway := grpc.NewChannelGatewayGRPCAPI(
		iampkg.Service(),
		gatewaypkg.GetApp(),
		gatewaypkg.GetConversation(),
		gatewaypkg.GetMessage(),
		gatewaypkg.GetRealtime(),
	)
	return []commonmodule.ApiRegInfo{
		commonmodule.NewApiRegInfo(gatewayv1.ChannelGatewayService_ServiceDesc, channelGateway),
	}
}

func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }

func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	channelHTTP := http.NewChannelGatewayHTTPAPI(
		iampkg.Service(),
		gatewaypkg.GetApp(),
		gatewaypkg.GetConversation(),
		gatewaypkg.GetMessage(),
		gatewaypkg.GetRealtime(),
	)
	routes := commonmodule.NewHttpRouteGroup("/api/v1").Compose(channelHTTP.Routes())
	return commonmodule.HttpApiInfo{
		Handlers:    routes.Handlers(),
		Middlewares: []commonmodule.HttpMiddleware{iampkg.AccountTokenMiddleware()},
	}
}
