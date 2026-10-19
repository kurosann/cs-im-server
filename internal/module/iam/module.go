package iam

import (
	"context"

	"cs-im-server/internal/cfg"
	"cs-im-server/internal/module/iam/internal/api"
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
	adminEmail := "admin@cs-im.local"
	adminPassword := "admin12345"
	if config := cfg.GetConfig(); config != nil {
		bootstrapToken = config.Access.AccountToken
		relayToken = config.Access.RelayToken
		tenantID = config.Access.TenantID
	}
	accountService := iamService.NewAccountService(bootstrapToken, tenantID, relayToken)
	pkg.SetService(accountService)
	pkg.SetStaffService(iamService.NewStaffService(accountService, adminEmail, adminPassword, tenantID))
	return nil
}

func (m *Module) Model() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{
		commonmodule.NewModelRegInfo(&model.AccountToken{}),
		commonmodule.NewModelRegInfo(&model.User{}),
		commonmodule.NewModelRegInfo(&model.ChannelCredential{}),
	}
}

func (m *Module) Api() []commonmodule.ApiRegInfo   { return nil }
func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }
func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	tokenAPI := api.NewTokenAPI(pkg.Service())
	authAPI := api.NewAuthAPI(pkg.StaffService())
	channelCredentialAPI := api.NewChannelCredentialAPI(pkg.Service())
	staffAPI := api.NewStaffAPI(pkg.StaffService())
	adminGroup := commonmodule.NewHttpRouteGroup("/admin/v1/iam", commonmodule.WithGroupMiddlewares(
		pkg.AccountTokenMiddleware(),
		pkg.RequirePrincipalScope(pkg.ScopeIAMTokenManage),
	)).Compose(tokenAPI.AdminRoutes(), channelCredentialAPI.Routes(), staffAPI.Routes())
	return commonmodule.HttpApiInfo{
		Handlers: append(adminGroup.Handlers(), authAPI.Routes()...),
	}
}
