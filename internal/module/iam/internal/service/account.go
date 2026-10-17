package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"cs-im-server/internal/module/iam/internal/model"
	"encoding/hex"
	"errors"

	"vane-common/pkg/dataaccess"
)

type Principal struct {
	AccountKey string
	TenantID   string
	Kind       string
}

type PrincipalContextKey struct{}

var ErrPrincipalMissing = errors.New("principal is missing")

type AccountService struct {
	accessor          dataaccess.DataAccessor[*model.AccountToken]
	bootstrapToken    string
	relayToken        string
	bootstrapTenantID string
}

type AccountTokenInfo struct {
	AccountKey string `json:"accountKey"`
	Name       string `json:"name"`
	TenantID   string `json:"tenantId"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
}

func NewAccountService(bootstrapToken, tenantID string, relayToken ...string) *AccountService {
	relayTokenValue := ""
	if len(relayToken) > 0 {
		relayTokenValue = relayToken[0]
	}
	return &AccountService{
		accessor:          dataaccess.DataAccessor[*model.AccountToken]{},
		bootstrapToken:    bootstrapToken,
		relayToken:        relayTokenValue,
		bootstrapTenantID: tenantID,
	}
}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, PrincipalContextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(PrincipalContextKey{}).(Principal)
	return principal, ok
}

func hashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func (s *AccountService) Authenticate(_ context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, unauthorized("account token is required")
	}
	item, err := s.accessor.First(context.Background(), dataaccess.Eq("token_hash", hashToken(token)))
	if err != nil && !errors.Is(err, dataaccess.ErrDataNotFound) {
		return Principal{}, err
	}
	if err == nil {
		if item.Status != "active" {
			return Principal{}, forbidden("account token is disabled")
		}
		kind := item.Kind
		if kind == "" {
			kind = PrincipalKindBusiness
		}
		return Principal{AccountKey: item.AccountKey, TenantID: item.TenantID, Kind: kind}, nil
	}
	if s.relayToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.relayToken)) == 1 {
		return Principal{AccountKey: "relay-account", TenantID: s.bootstrapTenantID, Kind: PrincipalKindRelay}, nil
	}
	if s.bootstrapToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.bootstrapToken)) != 1 {
		return Principal{}, unauthorized("invalid account token")
	}
	return Principal{AccountKey: "dev-account", TenantID: s.bootstrapTenantID, Kind: PrincipalKindBusiness}, nil
}

func (s *AccountService) Create(_ context.Context, token string, accountKey, name, tenantID string) (AccountTokenInfo, error) {
	if token == "" {
		return AccountTokenInfo{}, badRequest("token is required")
	}
	if accountKey == "" {
		return AccountTokenInfo{}, badRequest("accountKey is required")
	}
	if tenantID == "" {
		return AccountTokenInfo{}, badRequest("tenantId is required")
	}
	item := &model.AccountToken{
		AccountKey: accountKey,
		Name:       name,
		TenantID:   tenantID,
		Kind:       PrincipalKindBusiness,
		TokenHash:  hashToken(token),
		Status:     "active",
	}
	if item.Name == "" {
		item.Name = accountKey
	}
	if err := s.accessor.Save(context.Background(), item); err != nil {
		return AccountTokenInfo{}, err
	}
	return AccountTokenInfo{AccountKey: item.AccountKey, Name: item.Name, TenantID: item.TenantID, Kind: item.Kind, Status: item.Status}, nil
}
