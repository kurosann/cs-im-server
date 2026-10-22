package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	iamdto "cs-im-server/internal/module/iam/dto"
	"cs-im-server/internal/module/iam/internal/model"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"

	"vane-common/pkg/dataaccess"
)

const (
	ScopeAll                = "*"
	ScopeGatewayAdmin       = "gateway:admin"
	ScopeIAMTokenManage     = "iam:token:manage"
	ScopeIAMUserManage      = "iam:user:manage"
	ScopeIAMRoleManage      = "iam:role:manage"
	ScopeChannelAccess      = "channel:access"
	ScopeRelayAccess        = "relay:access"
	ScopeConversationRead   = "conversation:read"
	ScopeConversationManage = "conversation:manage"
	ScopeMessageSendAgent   = "message:send:agent"
	ScopeAuditRead          = "audit:read"
)

type AccountService struct {
	accessor    dataaccess.DataAccessor[*model.AccountToken]
	credentials dataaccess.DataAccessor[*model.ChannelCredential]
	users       dataaccess.DataAccessor[*model.User]
	roles       dataaccess.DataAccessor[*model.Role]
	audit       *AuditService
}

func NewAccountService() *AccountService {
	return NewAccountServiceWithAudit(NewAuditService())
}

func NewAccountServiceWithAudit(audit *AuditService) *AccountService {
	return &AccountService{
		accessor: dataaccess.DataAccessor[*model.AccountToken]{},
		roles:    dataaccess.DataAccessor[*model.Role]{},
		audit:    audit,
	}
}

func (s *AccountService) recordAudit(ctx context.Context, principal iamprincipal.Principal, event, action, object, result string) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, principal, iamdto.AuditEntry{Event: event, Action: action, Object: object, Result: result})
}

func hashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}

func (s *AccountService) Authenticate(_ context.Context, token string) (iamprincipal.Principal, error) {
	if token == "" {
		return iamprincipal.Principal{}, unauthorized("account token is required")
	}
	now := time.Now()
	item, err := s.accessor.First(context.Background(), dataaccess.Eq("token_hash", hashToken(token)))
	if err != nil && !errors.Is(err, dataaccess.ErrDataNotFound) {
		return iamprincipal.Principal{}, err
	}
	credential, err := s.credentials.First(context.Background(), dataaccess.Eq("secret_hash", hashToken(token)))
	if err != nil && !errors.Is(err, dataaccess.ErrDataNotFound) {
		return iamprincipal.Principal{}, err
	}
	if credential != nil {
		if credential.Status != "active" {
			return iamprincipal.Principal{}, forbidden("channel credential is disabled or revoked")
		}
		if credential.ExpiresAt != nil && credential.ExpiresAt.Before(now) {
			return iamprincipal.Principal{}, forbidden("channel credential is expired")
		}
		credential.LastUsedAt = &now
		if err := s.credentials.Update(context.Background(), credential); err != nil {
			return iamprincipal.Principal{}, err
		}
		return iamprincipal.Principal{
			AccountKey: credential.AppKey,
			Name:       credential.Name,
			TenantID:   credential.TenantID,
			Kind:       iamprincipal.KindBusiness,
			Scopes:     []string{ScopeChannelAccess},
		}, nil
	}
	if item != nil {
		if item.Status != "active" {
			return iamprincipal.Principal{}, forbidden("account token is disabled or revoked")
		}
		if item.ExpiresAt != nil && item.ExpiresAt.Before(now) {
			return iamprincipal.Principal{}, forbidden("account token is expired")
		}
		kind := item.Kind
		if kind == "" {
			kind = iamprincipal.KindBusiness
		}
		scopes, err := decodeScopes(item.ScopesJSON)
		if err != nil {
			return iamprincipal.Principal{}, err
		}
		item.LastUsedAt = &now
		if err := s.accessor.Update(context.Background(), item); err != nil {
			return iamprincipal.Principal{}, err
		}
		if kind == iamprincipal.KindHuman {
			return s.authenticateStaff(context.Background(), item)
		}
		return iamprincipal.Principal{
			AccountKey: item.AccountKey,
			Name:       item.Name,
			TenantID:   item.TenantID,
			Kind:       kind,
			Scopes:     scopes,
			TokenKey:   item.TokenKey,
		}, nil
	}
	return iamprincipal.Principal{}, unauthorized("invalid account token")
}

func (s *AccountService) authenticateStaff(ctx context.Context, item *model.AccountToken) (iamprincipal.Principal, error) {
	user, err := s.users.First(ctx, dataaccess.Eq("user_key", item.AccountKey), dataaccess.Eq("tenant_id", item.TenantID))
	if errors.Is(err, dataaccess.ErrDataNotFound) {
		return iamprincipal.Principal{}, unauthorized("staff account is missing")
	}
	if err != nil {
		return iamprincipal.Principal{}, err
	}
	if user.Status != "active" {
		return iamprincipal.Principal{}, forbidden("staff account is disabled")
	}
	scopes, err := s.roleScopes(ctx, user.TenantID, user.Role)
	if err != nil {
		return iamprincipal.Principal{}, err
	}
	return iamprincipal.Principal{
		AccountKey: user.UserKey,
		Name:       user.DisplayName,
		TenantID:   user.TenantID,
		Kind:       iamprincipal.KindHuman,
		Scopes:     scopes,
		TokenKey:   item.TokenKey,
	}, nil
}

func (s *AccountService) RoleScopes(ctx context.Context, tenantID, role string) ([]string, error) {
	return s.roleScopes(ctx, tenantID, role)
}

func (s *AccountService) roleScopes(ctx context.Context, tenantID, role string) ([]string, error) {
	if role == "owner" {
		return builtinRoleScopes(role), nil
	}
	item, err := s.roles.First(ctx, dataaccess.Eq("tenant_id", tenantID), dataaccess.Eq("role_key", role))
	if errors.Is(err, dataaccess.ErrDataNotFound) {
		if _, found := builtinRole(role); found {
			return builtinRoleScopes(role), nil
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if item.Status != "active" {
		return nil, nil
	}
	return decodeScopes(item.ScopesJSON)
}

func (s *AccountService) Issue(ctx context.Context, principal iamprincipal.Principal, req iamdto.IssueAccountTokenRequest) (iamdto.IssueAccountTokenResult, error) {
	if !principal.HasScope(ScopeIAMTokenManage) {
		return iamdto.IssueAccountTokenResult{}, forbidden("principal cannot manage account tokens")
	}
	if req.AccountKey == "" {
		return iamdto.IssueAccountTokenResult{}, badRequest("accountKey is required")
	}
	tenantID := principal.TenantID
	if req.TenantID != "" && req.TenantID != tenantID && !principal.HasScope(ScopeAll) {
		return iamdto.IssueAccountTokenResult{}, forbidden("principal cannot issue tokens for another tenant")
	}
	if req.TenantID != "" && principal.HasScope(ScopeAll) {
		tenantID = req.TenantID
	}
	kind := req.Kind
	if kind == "" {
		kind = iamprincipal.KindBusiness
	}
	if kind != iamprincipal.KindBusiness && kind != iamprincipal.KindRelay {
		return iamdto.IssueAccountTokenResult{}, badRequest("unsupported account token kind")
	}
	if kind == iamprincipal.KindRelay && principal.Kind != iamprincipal.KindRelay && !principal.HasScope(ScopeAll) {
		return iamdto.IssueAccountTokenResult{}, forbidden("principal cannot issue relay tokens")
	}
	scopes := cloneStrings(req.Scopes)
	if len(scopes) == 0 {
		scopes = []string{ScopeChannelAccess}
		if kind == iamprincipal.KindRelay {
			scopes = []string{ScopeRelayAccess}
		}
	}
	if !hasScopes(principal.Scopes, scopes) {
		return iamdto.IssueAccountTokenResult{}, forbidden("principal cannot issue requested scopes")
	}
	secret, err := generateToken("csim")
	if err != nil {
		return iamdto.IssueAccountTokenResult{}, err
	}
	tokenKey, err := generateTokenKey()
	if err != nil {
		return iamdto.IssueAccountTokenResult{}, err
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
		return iamdto.IssueAccountTokenResult{}, err
	}
	s.recordAudit(ctx, principal, "iam.token", "create", item.TokenKey, "success")
	return iamdto.IssueAccountTokenResult{Token: tokenInfo(item), Secret: secret}, nil
}

func (s *AccountService) List(ctx context.Context, principal iamprincipal.Principal) ([]iamdto.AccountTokenInfo, error) {
	if !principal.HasScope(ScopeIAMTokenManage) {
		return nil, forbidden("principal cannot manage account tokens")
	}
	items, err := s.accessor.List(ctx, dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return nil, err
	}
	result := make([]iamdto.AccountTokenInfo, 0, len(items))
	for _, item := range items {
		if item.Kind == iamprincipal.KindHuman {
			continue
		}
		result = append(result, tokenInfo(item))
	}
	return result, nil
}

func (s *AccountService) Get(ctx context.Context, principal iamprincipal.Principal, tokenKey string) (iamdto.AccountTokenInfo, error) {
	item, err := s.accessibleToken(ctx, principal, tokenKey)
	if err != nil {
		return iamdto.AccountTokenInfo{}, err
	}
	return tokenInfo(item), nil
}

func (s *AccountService) SetStatus(ctx context.Context, principal iamprincipal.Principal, tokenKey, status string) (iamdto.AccountTokenInfo, error) {
	if status != "active" && status != "disabled" {
		return iamdto.AccountTokenInfo{}, badRequest("unsupported account token status")
	}
	item, err := s.accessibleToken(ctx, principal, tokenKey)
	if err != nil {
		return iamdto.AccountTokenInfo{}, err
	}
	item.Status = status
	if err := s.accessor.Update(ctx, item); err != nil {
		return iamdto.AccountTokenInfo{}, err
	}
	s.recordAudit(ctx, principal, "iam.token", "status_change", item.TokenKey, "success")
	return tokenInfo(item), nil
}

func (s *AccountService) Rotate(ctx context.Context, principal iamprincipal.Principal, tokenKey string) (iamdto.IssueAccountTokenResult, error) {
	item, err := s.accessibleToken(ctx, principal, tokenKey)
	if err != nil {
		return iamdto.IssueAccountTokenResult{}, err
	}
	revokedAt := time.Now()
	item.RevokedAt = &revokedAt
	item.Status = "revoked"
	if err := s.accessor.Update(ctx, item); err != nil {
		return iamdto.IssueAccountTokenResult{}, err
	}
	s.recordAudit(ctx, principal, "iam.token", "rotate", item.TokenKey, "success")
	return s.Issue(ctx, principal, iamdto.IssueAccountTokenRequest{
		AccountKey: item.AccountKey,
		Name:       item.Name,
		Kind:       item.Kind,
		Scopes:     decodeScopesMust(item.ScopesJSON),
		ExpiresAt:  item.ExpiresAt,
	})
}

func (s *AccountService) accessibleToken(ctx context.Context, principal iamprincipal.Principal, tokenKey string) (*model.AccountToken, error) {
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

func tokenInfo(item *model.AccountToken) iamdto.AccountTokenInfo {
	scopes, _ := decodeScopes(item.ScopesJSON)
	return iamdto.AccountTokenInfo{
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
