package iam

import (
	"context"
	"sync"

	model "cs-im-server/internal/module/iam/internal/model"
	service "cs-im-server/internal/module/iam/internal/service"
)

type (
	Principal           = service.Principal
	PrincipalContextKey = service.PrincipalContextKey
	AccountService      = service.AccountService
	AccountTokenInfo    = service.AccountTokenInfo
	AccountToken        = model.AccountToken
)

var ErrPrincipalMissing = service.ErrPrincipalMissing

var serviceMu sync.RWMutex
var accountService *AccountService

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

func NewAccountService(bootstrapToken, tenantID string) *AccountService {
	return service.NewAccountService(bootstrapToken, tenantID)
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return service.WithPrincipal(ctx, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	return service.PrincipalFromContext(ctx)
}
