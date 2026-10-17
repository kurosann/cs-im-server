package iam

import (
	"context"

	"cs-im-server/internal/cfg"
	"cs-im-server/internal/module/iam/internal/model"
	iamService "cs-im-server/internal/module/iam/internal/service"
	pkg "cs-im-server/internal/module/iam/pkg"
	commonmodule "vane-common/module"
)

var _ commonmodule.IModule = (*Module)(nil)
var _ commonmodule.IModuleDb = (*Module)(nil)

type Module struct{}

func (m *Module) Init(_ context.Context) error {
	bootstrapToken := "dev-access-token"
	relayToken := "dev-relay-token"
	tenantID := "dev-tenant"
	if config := cfg.GetConfig(); config != nil {
		bootstrapToken = config.Access.AccountToken
		relayToken = config.Access.RelayToken
		tenantID = config.Access.TenantID
	}
	pkg.SetService(iamService.NewAccountService(bootstrapToken, tenantID, relayToken))
	return nil
}

func (m *Module) Model() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{commonmodule.NewModelRegInfo(&model.AccountToken{})}
}

func (m *Module) Api() []commonmodule.ApiRegInfo    { return nil }
func (m *Module) Job() []commonmodule.JobRegInfo    { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo  { return nil }
func (m *Module) HttpApi() commonmodule.HttpApiInfo { return commonmodule.HttpApiInfo{} }
