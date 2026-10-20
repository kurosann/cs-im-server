package iam

import (
	"context"
	"sync"

	iamdto "cs-im-server/internal/module/iam/dto"
	model "cs-im-server/internal/module/iam/internal/model"
	service "cs-im-server/internal/module/iam/internal/service"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
)

type (
	Principal                     = iamprincipal.Principal
	PrincipalContextKey           = iamprincipal.ContextKey
	AccountService                = service.AccountService
	AccountTokenInfo              = iamdto.AccountTokenInfo
	IssueAccountTokenRequest      = iamdto.IssueAccountTokenRequest
	IssueAccountTokenResult       = iamdto.IssueAccountTokenResult
	ChannelCredentialInfo         = iamdto.ChannelCredentialInfo
	IssueChannelCredentialRequest = iamdto.IssueChannelCredentialRequest
	IssueChannelCredentialResult  = iamdto.IssueChannelCredentialResult
	AccountToken                  = model.AccountToken
	ChannelCredential             = model.ChannelCredential
)

const (
	PrincipalKindBusiness = iamprincipal.KindBusiness
	PrincipalKindRelay    = iamprincipal.KindRelay
	ScopeAll              = service.ScopeAll
	ScopeGatewayAdmin     = service.ScopeGatewayAdmin
	ScopeIAMTokenManage   = service.ScopeIAMTokenManage
	ScopeChannelAccess    = service.ScopeChannelAccess
	ScopeRelayAccess      = service.ScopeRelayAccess
	PrincipalKindHuman    = iamprincipal.KindHuman
)

var ErrPrincipalMissing = iamprincipal.ErrMissing

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
	return iamprincipal.WithPrincipal(ctx, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	return iamprincipal.FromContext(ctx)
}
