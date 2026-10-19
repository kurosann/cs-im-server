package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"cs-im-server/internal/module/iam/internal/model"

	"vane-common/pkg/dataaccess"
)

const (
	ScopeAll            = "*"
	ScopeIAMTokenManage = "iam:token:manage"
	ScopeChannelAccess  = "channel:access"
	ScopeRelayAccess    = "relay:access"
)

type Principal struct {
	AccountKey string
	TenantID   string
	Kind       string
	Scopes     []string
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
	TokenKey   string     `json:"tokenKey"`
	AccountKey string     `json:"accountKey"`
	Name       string     `json:"name"`
	TenantID   string     `json:"tenantId"`
	Kind       string     `json:"kind"`
	Scopes     []string   `json:"scopes"`
	Status     string     `json:"status"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

type IssueAccountTokenRequest struct {
	AccountKey string     `json:"accountKey"`
	Name       string     `json:"name"`
	TenantID   string     `json:"tenantId"`
	Kind       string     `json:"kind"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expiresAt"`
}

type IssueAccountTokenResult struct {
	Token  AccountTokenInfo `json:"token"`
	Secret string           `json:"secret"`
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
	now := time.Now()
	if err == nil {
		if item.Status != "active" {
			return Principal{}, forbidden("account token is disabled or revoked")
		}
		if item.ExpiresAt != nil && item.ExpiresAt.Before(now) {
			return Principal{}, forbidden("account token is expired")
		}
		kind := item.Kind
		if kind == "" {
			kind = PrincipalKindBusiness
		}
		scopes, err := decodeScopes(item.ScopesJSON)
		if err != nil {
			return Principal{}, err
		}
		item.LastUsedAt = &now
		if err := s.accessor.Update(context.Background(), item); err != nil {
			return Principal{}, err
		}
		return Principal{AccountKey: item.AccountKey, TenantID: item.TenantID, Kind: kind, Scopes: scopes}, nil
	}
	if s.relayToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.relayToken)) == 1 {
		return Principal{AccountKey: "relay-account", TenantID: s.bootstrapTenantID, Kind: PrincipalKindRelay, Scopes: []string{ScopeAll}}, nil
	}
	if s.bootstrapToken == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.bootstrapToken)) != 1 {
		return Principal{}, unauthorized("invalid account token")
	}
	return Principal{AccountKey: "dev-account", TenantID: s.bootstrapTenantID, Kind: PrincipalKindBusiness, Scopes: []string{ScopeAll}}, nil
}

func (s *AccountService) Issue(ctx context.Context, principal Principal, req IssueAccountTokenRequest) (IssueAccountTokenResult, error) {
	if !principal.HasScope(ScopeIAMTokenManage) {
		return IssueAccountTokenResult{}, forbidden("principal cannot manage account tokens")
	}
	if req.AccountKey == "" {
		return IssueAccountTokenResult{}, badRequest("accountKey is required")
	}
	tenantID := principal.TenantID
	if req.TenantID != "" && req.TenantID != tenantID && !principal.HasScope(ScopeAll) {
		return IssueAccountTokenResult{}, forbidden("principal cannot issue tokens for another tenant")
	}
	if req.TenantID != "" && principal.HasScope(ScopeAll) {
		tenantID = req.TenantID
	}
	kind := req.Kind
	if kind == "" {
		kind = PrincipalKindBusiness
	}
	if kind != PrincipalKindBusiness && kind != PrincipalKindRelay {
		return IssueAccountTokenResult{}, badRequest("unsupported account token kind")
	}
	if kind == PrincipalKindRelay && principal.Kind != PrincipalKindRelay && !principal.HasScope(ScopeAll) {
		return IssueAccountTokenResult{}, forbidden("principal cannot issue relay tokens")
	}
	scopes := cloneStrings(req.Scopes)
	if len(scopes) == 0 {
		scopes = []string{ScopeChannelAccess}
		if kind == PrincipalKindRelay {
			scopes = []string{ScopeRelayAccess}
		}
	}
	if !hasScopes(principal.Scopes, scopes) {
		return IssueAccountTokenResult{}, forbidden("principal cannot issue requested scopes")
	}
	secret, err := generateToken("csim")
	if err != nil {
		return IssueAccountTokenResult{}, err
	}
	tokenKey, err := generateTokenKey()
	if err != nil {
		return IssueAccountTokenResult{}, err
	}
	item := &model.AccountToken{
		TokenKey:   tokenKey,
		AccountKey: req.AccountKey,
		Name:       req.Name,
		TenantID:   tenantID,
		Kind:       kind,
		ScopesJSON: encodeScopes(scopes),
		TokenHash:  hashToken(secret),
		Status:     "active",
		ExpiresAt:  req.ExpiresAt,
	}
	if item.Name == "" {
		item.Name = req.AccountKey
	}
	if err := s.accessor.Create(ctx, item); err != nil {
		return IssueAccountTokenResult{}, err
	}
	return IssueAccountTokenResult{Token: tokenInfo(item), Secret: secret}, nil
}

func (s *AccountService) List(ctx context.Context, principal Principal) ([]AccountTokenInfo, error) {
	if !principal.HasScope(ScopeIAMTokenManage) {
		return nil, forbidden("principal cannot manage account tokens")
	}
	items, err := s.accessor.List(ctx, dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return nil, err
	}
	result := make([]AccountTokenInfo, 0, len(items))
	for _, item := range items {
		result = append(result, tokenInfo(item))
	}
	return result, nil
}

func (s *AccountService) Get(ctx context.Context, principal Principal, tokenKey string) (AccountTokenInfo, error) {
	item, err := s.accessibleToken(ctx, principal, tokenKey)
	if err != nil {
		return AccountTokenInfo{}, err
	}
	return tokenInfo(item), nil
}

func (s *AccountService) SetStatus(ctx context.Context, principal Principal, tokenKey, status string) (AccountTokenInfo, error) {
	if status != "active" && status != "disabled" {
		return AccountTokenInfo{}, badRequest("unsupported account token status")
	}
	item, err := s.accessibleToken(ctx, principal, tokenKey)
	if err != nil {
		return AccountTokenInfo{}, err
	}
	item.Status = status
	if err := s.accessor.Update(ctx, item); err != nil {
		return AccountTokenInfo{}, err
	}
	return tokenInfo(item), nil
}

func (s *AccountService) Rotate(ctx context.Context, principal Principal, tokenKey string) (IssueAccountTokenResult, error) {
	item, err := s.accessibleToken(ctx, principal, tokenKey)
	if err != nil {
		return IssueAccountTokenResult{}, err
	}
	revokedAt := time.Now()
	item.RevokedAt = &revokedAt
	item.Status = "revoked"
	if err := s.accessor.Update(ctx, item); err != nil {
		return IssueAccountTokenResult{}, err
	}
	return s.Issue(ctx, principal, IssueAccountTokenRequest{
		AccountKey: item.AccountKey,
		Name:       item.Name,
		Kind:       item.Kind,
		Scopes:     decodeScopesMust(item.ScopesJSON),
		ExpiresAt:  item.ExpiresAt,
	})
}

func (s *AccountService) accessibleToken(ctx context.Context, principal Principal, tokenKey string) (*model.AccountToken, error) {
	if !principal.HasScope(ScopeIAMTokenManage) {
		return nil, forbidden("principal cannot manage account tokens")
	}
	item, err := s.accessor.First(ctx, dataaccess.Eq("token_key", tokenKey))
	if err != nil {
		return nil, notFound("account token not found")
	}
	if item.TenantID != principal.TenantID && !principal.HasScope(ScopeAll) {
		return nil, forbidden("account token is not accessible")
	}
	return item, nil
}

func tokenInfo(item *model.AccountToken) AccountTokenInfo {
	scopes, _ := decodeScopes(item.ScopesJSON)
	return AccountTokenInfo{
		TokenKey:   item.TokenKey,
		AccountKey: item.AccountKey,
		Name:       item.Name,
		TenantID:   item.TenantID,
		Kind:       item.Kind,
		Scopes:     scopes,
		Status:     item.Status,
		ExpiresAt:  item.ExpiresAt,
		RevokedAt:  item.RevokedAt,
		LastUsedAt: item.LastUsedAt,
	}
}

func encodeScopes(scopes []string) string {
	data, _ := json.Marshal(cloneStrings(scopes))
	return string(data)
}

func decodeScopes(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var scopes []string
	if err := json.Unmarshal([]byte(value), &scopes); err != nil {
		return nil, badRequest("invalid account token scopes")
	}
	return cloneStrings(scopes), nil
}

func decodeScopesMust(value string) []string {
	scopes, _ := decodeScopes(value)
	return scopes
}

func hasScopes(granted, requested []string) bool {
	for _, scope := range requested {
		if !containsScope(granted, scope) {
			return false
		}
	}
	return true
}

func containsScope(scopes []string, scope string) bool {
	for _, value := range scopes {
		if value == scope || value == ScopeAll {
			return true
		}
	}
	return false
}

func generateToken(prefix string) (string, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(secret), nil
}

func generateTokenKey() (string, error) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return "tok_" + hex.EncodeToString(key), nil
}
