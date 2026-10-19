package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"cs-im-server/internal/module/iam/internal/model"

	"vane-common/pkg/dataaccess"

	"golang.org/x/crypto/bcrypt"
)

const sessionTTL = 7 * 24 * time.Hour

type StaffService struct {
	users          dataaccess.DataAccessor[*model.User]
	accounts       *AccountService
	adminEmail     string
	adminPassword  string
	bootstrapTenan string
}

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type LoginResult struct {
	User  User  `json:"user"`
	Token Token `json:"token"`
}

type User struct {
	UserKey     string     `json:"userKey"`
	Account     string     `json:"account"`
	Email       string     `json:"email"`
	DisplayName string     `json:"displayName"`
	TenantID    string     `json:"tenantId"`
	Role        string     `json:"role"`
	Status      string     `json:"status"`
	LastLoginAt *time.Time `json:"lastLoginAt,omitempty"`
}

type Token struct {
	Token     string     `json:"token"`
	TokenKey  string     `json:"tokenKey"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

type CreateStaffRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

type CreateStaffResult struct {
	User            User   `json:"user"`
	InitialPassword string `json:"initialPassword"`
}

func NewStaffService(accounts *AccountService, adminEmail, adminPassword, tenantID string) *StaffService {
	return &StaffService{
		users:          dataaccess.DataAccessor[*model.User]{},
		accounts:       accounts,
		adminEmail:     strings.ToLower(strings.TrimSpace(adminEmail)),
		adminPassword:  adminPassword,
		bootstrapTenan: tenantID,
	}
}

func (s *StaffService) Login(ctx context.Context, req LoginRequest) (LoginResult, error) {
	identifier := strings.ToLower(strings.TrimSpace(req.Identifier))
	if identifier == "" || req.Password == "" {
		return LoginResult{}, badRequest("identifier and password are required")
	}
	if utf8.RuneCountInString(req.Password) < 8 {
		return LoginResult{}, unauthorized("invalid identifier or password")
	}
	user, err := s.findUser(ctx, identifier)
	if err != nil {
		return LoginResult{}, unauthorized("invalid identifier or password")
	}
	if user.Status != "active" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return LoginResult{}, unauthorized("invalid identifier or password")
	}
	now := time.Now()
	expiresAt := now.Add(sessionTTL)
	secret, err := generateToken("csim")
	if err != nil {
		return LoginResult{}, err
	}
	tokenKey, err := generateTokenKey()
	if err != nil {
		return LoginResult{}, err
	}
	session := &model.AccountToken{
		TokenKey:   tokenKey,
		AccountKey: user.UserKey,
		Name:       user.DisplayName,
		TenantID:   user.TenantID,
		Kind:       PrincipalKindHuman,
		ScopesJSON: encodeScopes(roleScopes(user.Role)),
		TokenHash:  hashToken(secret),
		Status:     "active",
		ExpiresAt:  &expiresAt,
	}
	if err := s.accounts.accessor.Create(ctx, session); err != nil {
		return LoginResult{}, err
	}
	user.LastLoginAt = &now
	if err := s.users.Update(ctx, user); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{User: staffInfo(user), Token: Token{Token: secret, TokenKey: tokenKey, ExpiresAt: &expiresAt}}, nil
}

func (s *StaffService) Logout(ctx context.Context, principal Principal) error {
	if principal.Kind != PrincipalKindHuman || principal.TokenKey == "" {
		return forbidden("only staff sessions can be logged out")
	}
	session, err := s.accounts.accessor.First(ctx, dataaccess.Eq("token_key", principal.TokenKey), dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return err
	}
	if session.AccountKey != principal.AccountKey || session.Kind != PrincipalKindHuman {
		return forbidden("session does not belong to this account")
	}
	now := time.Now()
	session.Status = "revoked"
	session.RevokedAt = &now
	return s.accounts.accessor.Update(ctx, session)
}

func (s *StaffService) List(ctx context.Context, principal Principal) ([]User, error) {
	if err := s.requireUserManager(principal); err != nil {
		return nil, err
	}
	items, err := s.users.List(ctx, dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return nil, err
	}
	result := make([]User, 0, len(items))
	for _, item := range items {
		result = append(result, staffInfo(item))
	}
	return result, nil
}

func (s *StaffService) Create(ctx context.Context, principal Principal, req CreateStaffRequest) (CreateStaffResult, error) {
	if err := s.requireUserManager(principal); err != nil {
		return CreateStaffResult{}, err
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" || email == "" {
		return CreateStaffResult{}, badRequest("displayName and email are required")
	}
	if req.Role != "admin" && req.Role != "supervisor" && req.Role != "agent" && req.Role != "auditor" && req.Role != "developer" {
		return CreateStaffResult{}, badRequest("unsupported staff role")
	}
	if _, err := s.users.First(ctx, dataaccess.Eq("email", email)); err == nil {
		return CreateStaffResult{}, badRequest("staff email already exists")
	} else if !errors.Is(err, dataaccess.ErrDataNotFound) {
		return CreateStaffResult{}, err
	}
	password, err := generateToken("pwd")
	if err != nil {
		return CreateStaffResult{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return CreateStaffResult{}, err
	}
	userKey, err := generateUserKey()
	if err != nil {
		return CreateStaffResult{}, err
	}
	account, err := generateStaffAccount()
	if err != nil {
		return CreateStaffResult{}, err
	}
	item := &model.User{
		UserKey:      userKey,
		Account:      account,
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		TenantID:     principal.TenantID,
		Role:         req.Role,
		Status:       "active",
	}
	if err := s.users.Create(ctx, item); err != nil {
		return CreateStaffResult{}, err
	}
	return CreateStaffResult{User: staffInfo(item), InitialPassword: password}, nil
}

func (s *StaffService) SetRole(ctx context.Context, principal Principal, userKey, role string) (User, error) {
	item, err := s.accessStaff(ctx, principal, userKey)
	if err != nil {
		return User{}, err
	}
	if role != "admin" && role != "supervisor" && role != "agent" && role != "auditor" && role != "developer" {
		return User{}, badRequest("unsupported staff role")
	}
	item.Role = role
	if err := s.users.Update(ctx, item); err != nil {
		return User{}, err
	}
	return staffInfo(item), nil
}

func (s *StaffService) SetStatus(ctx context.Context, principal Principal, userKey, status string) (User, error) {
	item, err := s.accessStaff(ctx, principal, userKey)
	if err != nil {
		return User{}, err
	}
	if status != "active" && status != "disabled" {
		return User{}, badRequest("unsupported staff status")
	}
	item.Status = status
	if err := s.users.Update(ctx, item); err != nil {
		return User{}, err
	}
	if status == "disabled" {
		if err := s.revokeSessions(ctx, item); err != nil {
			return User{}, err
		}
	}
	return staffInfo(item), nil
}

func (s *StaffService) accessStaff(ctx context.Context, principal Principal, userKey string) (*model.User, error) {
	if err := s.requireUserManager(principal); err != nil {
		return nil, err
	}
	if userKey == "" {
		return nil, badRequest("userKey is required")
	}
	item, err := s.users.First(ctx, dataaccess.Eq("user_key", userKey), dataaccess.Eq("tenant_id", principal.TenantID))
	if errors.Is(err, dataaccess.ErrDataNotFound) {
		return nil, notFound("staff not found")
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func (s *StaffService) revokeSessions(ctx context.Context, user *model.User) error {
	sessions, err := s.accounts.accessor.List(ctx, dataaccess.Eq("account_key", user.UserKey), dataaccess.Eq("tenant_id", user.TenantID), dataaccess.Eq("kind", PrincipalKindHuman))
	if err != nil {
		return err
	}
	now := time.Now()
	for _, session := range sessions {
		if session.Status != "active" {
			continue
		}
		session.Status = "revoked"
		session.RevokedAt = &now
		if err := s.accounts.accessor.Update(ctx, session); err != nil {
			return err
		}
	}
	return nil
}

func (s *StaffService) requireUserManager(principal Principal) error {
	if principal.Kind != PrincipalKindHuman {
		return forbidden("only staff can manage users")
	}
	if !principal.HasScope(ScopeIAMUserManage) {
		return forbidden("principal cannot manage users")
	}
	return nil
}

func (s *StaffService) Accounts() *AccountService {
	return s.accounts
}

func (s *StaffService) findUser(ctx context.Context, identifier string) (*model.User, error) {
	user, err := s.users.First(ctx, dataaccess.Eq("email", identifier))
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, dataaccess.ErrDataNotFound) {
		return nil, err
	}
	if identifier == s.adminEmail {
		if err := s.ensureAdminUser(ctx); err != nil {
			return nil, err
		}
		return s.users.First(ctx, dataaccess.Eq("email", identifier))
	}
	return s.users.First(ctx, dataaccess.Eq("account", identifier))
}

func (s *StaffService) ensureAdminUser(ctx context.Context) error {
	existing, err := s.users.List(ctx)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.adminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	userKey, err := generateUserKey()
	if err != nil {
		return err
	}
	admin := &model.User{
		UserKey:      userKey,
		Account:      "admin",
		Email:        s.adminEmail,
		DisplayName:  "系统管理员",
		PasswordHash: string(hash),
		TenantID:     s.bootstrapTenan,
		Role:         "admin",
		Status:       "active",
	}
	return s.users.Create(ctx, admin)
}

func generateUserKey() (string, error) {
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return "usr_" + hex.EncodeToString(key), nil
}

func generateStaffAccount() (string, error) {
	key := make([]byte, 8)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return "staff_" + hex.EncodeToString(key), nil
}

func roleScopes(role string) []string {
	switch role {
	case "admin":
		return []string{ScopeGatewayAdmin, ScopeIAMTokenManage, ScopeIAMUserManage, "conversation:read", "conversation:manage", "message:send:agent", "audit:read"}
	case "supervisor":
		return []string{ScopeGatewayAdmin, "conversation:read", "conversation:manage", "message:send:agent", "audit:read"}
	case "auditor":
		return []string{"audit:read"}
	case "developer":
		return []string{"channel:access", "conversation:read", "audit:read"}
	default:
		return []string{ScopeGatewayAdmin, "conversation:read", "conversation:manage", "message:send:agent"}
	}
}

func staffInfo(user *model.User) User {
	return User{
		UserKey:     user.UserKey,
		Account:     user.Account,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		TenantID:    user.TenantID,
		Role:        user.Role,
		Status:      user.Status,
		LastLoginAt: user.LastLoginAt,
	}
}
