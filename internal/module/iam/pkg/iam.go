package iam

import (
	"context"
	"sync"

	model "cs-im-server/internal/module/iam/internal/model"
	service "cs-im-server/internal/module/iam/internal/service"
)

type (
	Principal                     = service.Principal
	PrincipalContextKey           = service.PrincipalContextKey
	AccountService                = service.AccountService
	AccountTokenInfo              = service.AccountTokenInfo
	IssueAccountTokenRequest      = service.IssueAccountTokenRequest
	IssueAccountTokenResult       = service.IssueAccountTokenResult
	ChannelCredentialInfo         = service.ChannelCredentialInfo
	IssueChannelCredentialRequest = service.IssueChannelCredentialRequest
	IssueChannelCredentialResult  = service.IssueChannelCredentialResult
	AccountToken                  = model.AccountToken
	ChannelCredential             = model.ChannelCredential
)

const (
	PrincipalKindBusiness = service.PrincipalKindBusiness
	PrincipalKindRelay    = service.PrincipalKindRelay
	ScopeAll              = service.ScopeAll
	ScopeGatewayAdmin     = service.ScopeGatewayAdmin
	ScopeIAMTokenManage   = service.ScopeIAMTokenManage
	ScopeChannelAccess    = service.ScopeChannelAccess
	ScopeRelayAccess      = service.ScopeRelayAccess
	PrincipalKindHuman    = service.PrincipalKindHuman
)

var ErrPrincipalMissing = service.ErrPrincipalMissing

var serviceMu sync.RWMutex
var accountService *AccountService
var staffService *service.StaffService

func SetService(next *AccountService) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	accountService = next
}

func Service() *AccountService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if accountService == nil {
		panic("iam service is not initialized")
	}
	return accountService
}

func SetStaffService(next *service.StaffService) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	staffService = next
}

func StaffService() *service.StaffService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if staffService == nil {
		panic("iam staff service is not initialized")
	}
	return staffService
}

func NewAccountService(bootstrapToken, tenantID string) *AccountService {
	return service.NewAccountService(bootstrapToken, tenantID)
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return service.WithPrincipal(ctx, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	return service.PrincipalFromContext(ctx)
}
