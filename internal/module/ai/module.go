package ai

import (
	"context"

	aihttp "cs-im-server/internal/module/ai/internal/api"
	model "cs-im-server/internal/module/ai/internal/model"
	aiService "cs-im-server/internal/module/ai/internal/service"

	iamhttp "cs-im-server/internal/module/iam/pkg/http"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"

	commonmodule "vane-common/module"
)

var _ commonmodule.IModule = (*Module)(nil)
var _ commonmodule.IHTTPModule = (*Module)(nil)
var _ commonmodule.IModuleDb = (*Module)(nil)

type Module struct {
	accounts  *iampkg.AccountService
	providers *aiService.ProviderService
}

func (m *Module) Init(_ context.Context) error {
	m.accounts = iampkg.Service()
	m.providers = aiService.NewProviderService()
	if err := m.providers.Load(); err != nil {
		return err
	}
	return nil
}

func (m *Module) Model() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{
		commonmodule.NewModelRegInfo(&model.AIProvider{}),
	}
}

func (m *Module) Api() []commonmodule.ApiRegInfo   { return nil }
func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }

func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	providerAPI := aihttp.NewProviderAPI(m.providers)
	providerRoutes := providerAPI.Routes()
	return commonmodule.HttpApiInfo{
		Handlers: providerRoutes.Handlers(),
		Middlewares: []commonmodule.HttpMiddleware{
			iamhttp.AccountTokenMiddleware(m.accounts),
			iamhttp.RequirePrincipalKind(iampkg.PrincipalKindHuman),
			iamhttp.RequirePrincipalScope(iampkg.ScopeGatewayAdmin),
		},
	}
}
