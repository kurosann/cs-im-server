package iam

import (
	"context"

	iamhttp "cs-im-server/internal/module/iam/pkg/http"

	"cs-im-server/internal/module/iam/internal/api"
	"cs-im-server/internal/module/iam/internal/model"
	iamService "cs-im-server/internal/module/iam/internal/service"
	"cs-im-server/internal/module/iam/pkg/iam"
	commonmodule "vane-common/module"
)

var (
	_ commonmodule.IModule   = (*Module)(nil)
	_ commonmodule.IModuleDb = (*Module)(nil)
)

type Module struct {
	accounts *iamService.AccountService
	staff    *iamService.StaffService
	roles    *iamService.RoleService
	audit    *iamService.AuditService
}

func (m *Module) Init(_ context.Context) error {
	m.audit = iamService.NewAuditService()
	m.accounts = iamService.NewAccountServiceWithAudit(m.audit)
	m.staff = iamService.NewStaffService(m.accounts)
	m.roles = iamService.NewRoleService(m.audit)
	iam.SetService(m.accounts)
	iam.SetStaffService(m.staff)
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
	tokenAPI := api.NewTokenAPI(m.accounts)
	authAPI := api.NewAuthAPI(m.accounts, m.staff)
	channelCredentialAPI := api.NewChannelCredentialAPI(m.accounts)
	staffAPI := api.NewStaffAPI(m.staff)
	roleAPI := api.NewRoleAPI(m.roles)
	adminGroup := commonmodule.NewHttpRouteGroup("/admin/v1/iam", commonmodule.WithGroupMiddlewares(
		iamhttp.AccountTokenMiddleware(m.accounts),
		iamhttp.RequirePrincipalScope(iam.ScopeIAMTokenManage),
	)).Compose(tokenAPI.AdminRoutes(), channelCredentialAPI.Routes(), staffAPI.Routes())
	roleGroup := commonmodule.NewHttpRouteGroup("/admin/v1/iam", commonmodule.WithGroupMiddlewares(
		iamhttp.AccountTokenMiddleware(m.accounts),
		iamhttp.RequirePrincipalScope(iam.ScopeIAMRoleManage),
	)).Compose(roleAPI.Routes())
	auditAPI := api.NewAuditAPI(m.audit)
	auditGroup := commonmodule.NewHttpRouteGroup("/admin/v1/iam", commonmodule.WithGroupMiddlewares(
		iamhttp.AccountTokenMiddleware(m.accounts),
		iamhttp.RequirePrincipalScope(iam.ScopeAuditRead),
	)).Compose(auditAPI.Routes())

	return commonmodule.HttpApiInfo{
		Handlers: append(authAPI.Routes(), adminGroup.Merge(roleGroup, auditGroup)...),
	}
}
