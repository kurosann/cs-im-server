package iam

import (
	"context"
	iamhttp "cs-im-server/internal/module/iam/pkg/http"

	"cs-im-server/internal/cfg"
	"cs-im-server/internal/module/iam/internal/api"
	"cs-im-server/internal/module/iam/internal/model"
	iamService "cs-im-server/internal/module/iam/internal/service"
	"cs-im-server/internal/module/iam/pkg/iam"
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
	iam.SetService(accountService)
	iam.SetStaffService(iamService.NewStaffService(accountService, adminEmail, adminPassword, tenantID))
	return nil
}

func (m *Module) Model() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{
		commonmodule.NewModelRegInfo(&model.AccountToken{}),
		commonmodule.NewModelRegInfo(&model.User{}),
		commonmodule.NewModelRegInfo(&model.Tenant{}),
		commonmodule.NewModelRegInfo(&model.Role{}),
		commonmodule.NewModelRegInfo(&model.ChannelCredential{}),
		commonmodule.NewModelRegInfo(&model.AuditLog{}),
	}
}

func (m *Module) Api() []commonmodule.ApiRegInfo   { return nil }
func (m *Module) Job() []commonmodule.JobRegInfo   { return nil }
func (m *Module) Task() []commonmodule.TaskRegInfo { return nil }
func (m *Module) HttpApi() commonmodule.HttpApiInfo {
	tokenAPI := api.NewTokenAPI(iam.Service())
	authAPI := api.NewAuthAPI(iam.StaffService())
	channelCredentialAPI := api.NewChannelCredentialAPI(iam.Service())
	staffAPI := api.NewStaffAPI(iam.StaffService())
	roleAPI := api.NewRoleAPI(iamService.NewRoleService(iamService.NewAuditService()))
	adminGroup := commonmodule.NewHttpRouteGroup("/admin/v1/iam", commonmodule.WithGroupMiddlewares(
		iamhttp.AccountTokenMiddleware(),
		iamhttp.RequirePrincipalScope(iam.ScopeIAMTokenManage),
	)).Compose(tokenAPI.AdminRoutes(), channelCredentialAPI.Routes(), staffAPI.Routes())
	roleGroup := commonmodule.NewHttpRouteGroup("/admin/v1/iam", commonmodule.WithGroupMiddlewares(
		iamhttp.AccountTokenMiddleware(),
		iamhttp.RequirePrincipalScope(iamService.ScopeIAMRoleManage),
	)).Compose(roleAPI.Routes())
	auditAPI := api.NewAuditAPI(iamService.NewAuditService())
	auditGroup := commonmodule.NewHttpRouteGroup("/admin/v1/iam", commonmodule.WithGroupMiddlewares(
		iamhttp.AccountTokenMiddleware(),
		iamhttp.RequirePrincipalScope("audit:read"),
	)).Compose(auditAPI.Routes())
	return commonmodule.HttpApiInfo{
		Handlers: append(adminGroup.Handlers(), append(authAPI.Routes(), append(roleGroup.Handlers(), auditGroup.Handlers()...)...)...),
	}
}
