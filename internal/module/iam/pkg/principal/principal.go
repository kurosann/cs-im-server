package principal

import (
	"context"
	"errors"
)

const (
	KindBusiness = "business"
	KindRelay    = "relay"
	KindHuman    = "human"
)

type Principal struct {
	AccountKey string
	Name       string
	TenantID   string
	Kind       string
	Scopes     []string
	TokenKey   string
}

type contextKey struct{}

type ValueSetter interface {
	SetValue(key, value any)
}

const scopeWildcard = "*"

var ErrMissing = errors.New("principal is missing")

func SetValue(ctx ValueSetter, principal Principal) {
	ctx.SetValue(contextKey{}, principal)
}

func FromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(contextKey{}).(Principal)
	return principal, ok
}

func (p Principal) HasScope(scope string) bool {
	if scope == "" {
		return false
	}
	for _, value := range p.Scopes {
		if value == scope || value == scopeWildcard {
			return true
		}
	}
	return false
}

func (p Principal) HasKind(kind string) bool {
	if p.Kind == "" {
		return kind == KindBusiness
	}
	return p.Kind == kind
}
