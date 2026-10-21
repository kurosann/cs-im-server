package llm

import (
	"context"
	"os"

	iamhttp "cs-im-server/internal/module/iam/pkg/http"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/internal/module/llm/internal/api"
	"cs-im-server/internal/module/llm/internal/config"
	"cs-im-server/internal/module/llm/internal/service"
	llmpkg "cs-im-server/internal/module/llm/pkg"
	commonmodule "vane-common/module"
)

var _ commonmodule.IModule = (*Module)(nil)
var _ commonmodule.IHTTPModule = (*Module)(nil)

type Module struct {
	service    *service.Service
	configPath string
}

func NewModule(configPath string) *Module { return &Module{configPath: configPath} }

func (m *Module) Init(_ context.Context) error {
	path := m.configPath
	if path == "" {
		path = os.Getenv("CS_IM_LLM_CONFIG")
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	m.service = service.New(cfg, nil)
	llmpkg.Register(moduleClient{service: m.service})
	return nil
}

type moduleClient struct{ service *service.Service }

func (c moduleClient) TranslationSelector() string { return c.service.TranslationSelector() }

func (c moduleClient) TranslateText(ctx context.Context, text, sourceLang, targetLang string) (string, error) {
	return c.service.Translate(ctx, text, sourceLang, targetLang)
}

func (m *Module) Api() []commonmodule.ApiRegInfo   { return nil }
func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }

func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	if m.service == nil {
		return commonmodule.HttpApiInfo{}
	}
	llmAPI := api.New(m.service)
	routes := llmAPI.Routes()
	return commonmodule.HttpApiInfo{
		Prefix:   "/v1",
		Handlers: routes.Handlers(),
		Middlewares: []commonmodule.HttpMiddleware{
			iamhttp.AccountTokenMiddleware(),
			iamhttp.RequirePrincipalKind(iampkg.PrincipalKindHuman),
			iamhttp.RequirePrincipalScope(iampkg.ScopeGatewayAdmin),
		},
	}
}
