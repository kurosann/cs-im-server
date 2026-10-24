package translation

import (
	"context"

	translationhttp "cs-im-server/internal/module/translation/internal/api"
	model "cs-im-server/internal/module/translation/internal/model"
	translationService "cs-im-server/internal/module/translation/internal/service"
	translationpkg "cs-im-server/internal/module/translation/pkg/translation"

	aipkg "cs-im-server/internal/module/ai/pkg/ai"
	iamhttp "cs-im-server/internal/module/iam/pkg/http"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"

	commonmodule "vane-common/module"
)

var (
	_ commonmodule.IModule     = (*Module)(nil)
	_ commonmodule.IHTTPModule = (*Module)(nil)
	_ commonmodule.IModuleDb   = (*Module)(nil)
)

type Module struct {
	providers *translationService.TranslationService
}

func (m *Module) Init(_ context.Context) error {
	m.providers = translationService.NewTranslationService(aipkg.Service())
	translationpkg.SetService(m.providers)
	return nil
}

func (m *Module) Model() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{
		commonmodule.NewModelRegInfo(&model.Provider{}),
		commonmodule.NewModelRegInfo(&model.Cache{}),
	}
}

func (m *Module) Api() []commonmodule.ApiRegInfo   { return nil }
func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }

func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	providerAPI := translationhttp.NewProviderAPI(m.providers)
	adminGroup := commonmodule.NewHttpRouteGroup("/admin/v1/translation", commonmodule.WithGroupMiddlewares(
		iamhttp.AccountTokenMiddleware(iampkg.Service()),
		iamhttp.RequirePrincipalScope(iampkg.ScopeGatewayAdmin),
	)).Compose(providerAPI.Routes(), providerAPI.AdminRoutes())
	return commonmodule.HttpApiInfo{
		Handlers: adminGroup.Handlers(),
	}
}
