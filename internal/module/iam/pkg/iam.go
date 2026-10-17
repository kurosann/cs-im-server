package iam

import (
	"context"
	"sync"

	service "cs-im-server/internal/module/iam/internal/service"
)

type (
	Principal           = service.Principal
	PrincipalContextKey = service.PrincipalContextKey
	AccountService      = service.AccountService
	AccountTokenInfo    = service.AccountTokenInfo
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

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return service.WithPrincipal(ctx, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	return service.PrincipalFromContext(ctx)
}
