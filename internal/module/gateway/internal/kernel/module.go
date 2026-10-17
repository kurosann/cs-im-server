package kernel

import (
	"context"

	iampkg "cs-im-server/internal/module/iam/pkg"

	apppkg "cs-im-server/internal/module/gateway/internal/app/pkg"
	conversationpkg "cs-im-server/internal/module/gateway/internal/conversation/pkg"
	customerpkg "cs-im-server/internal/module/gateway/internal/customer/pkg"
	messagepkg "cs-im-server/internal/module/gateway/internal/message/pkg"
	realtimepkg "cs-im-server/internal/module/gateway/internal/realtime/pkg"

	commonmodule "vane-common/module"
)

var _ commonmodule.IModule = (*Module)(nil)
var _ commonmodule.IHTTPModule = (*Module)(nil)
var _ commonmodule.IModuleDb = (*Module)(nil)

type Module struct{}

func (m *Module) Init(_ context.Context) error {
	realtimepkg.Init()
	if err := apppkg.Init(realtimepkg.Service()); err != nil {
		return err
	}
	if err := customerpkg.Init(realtimepkg.Service()); err != nil {
		return err
	}
	if err := conversationpkg.Init(apppkg.Service(), customerpkg.Service(), realtimepkg.Service()); err != nil {
		return err
	}
	if err := messagepkg.Init(apppkg.Service(), conversationpkg.Service(), realtimepkg.Service()); err != nil {
		return err
	}
	conversationpkg.SetMessages(messagepkg.NewConversationMessageStore())
	return nil
}

func (m *Module) Model() []commonmodule.ModelRegInfo {
	models := apppkg.Models()
	models = append(models, customerpkg.Models()...)
	models = append(models, conversationpkg.Models()...)
	models = append(models, messagepkg.Models()...)
	return models
}

func (m *Module) Api() []commonmodule.ApiRegInfo   { return nil }
func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }

func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	apiGroup := commonmodule.NewHttpRouteGroup("/api/v1").
		Compose(
			apppkg.RuntimeRoutes(messagepkg.Service()),
			customerpkg.Routes(),
			conversationpkg.Routes(),
			messagepkg.Routes(),
			messagepkg.ConversationRoutes(),
		)
	adminGroup := commonmodule.NewHttpRouteGroup("/admin/v1").
		Compose(
			apppkg.AdminRoutes(messagepkg.Service()),
			conversationpkg.AdminRoutes(),
		)
	realtimeGroup := commonmodule.NewHttpRouteGroup("/ws/v1").Compose(realtimepkg.Routes())

	return commonmodule.HttpApiInfo{
		Handlers:    commonmodule.MergeHandlers(apiGroup.Handlers(), adminGroup.Handlers(), realtimeGroup.Handlers()),
		Middlewares: []commonmodule.HttpMiddleware{iampkg.AccountTokenMiddleware()},
	}
}
