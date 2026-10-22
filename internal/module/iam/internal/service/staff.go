package service

import (
	"context"
	"crypto/rand"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	iamdto "cs-im-server/internal/module/iam/dto"
	"cs-im-server/internal/module/iam/internal/model"

	"vane-common/pkg/dataaccess"
	damodel "vane-common/pkg/dataaccess/model"

	"golang.org/x/crypto/bcrypt"
)

const sessionTTL = 7 * 24 * time.Hour

type StaffService struct {
	users    dataaccess.DataAccessor[*model.User]
	roles    dataaccess.DataAccessor[*model.Role]
	tenants  dataaccess.DataAccessor[*model.Tenant]
	accounts *AccountService
	audit    *AuditService
}

func NewStaffService(accounts *AccountService) *StaffService {
	return &StaffService{
		users:    dataaccess.DataAccessor[*model.User]{},
		roles:    dataaccess.DataAccessor[*model.Role]{},
		tenants:  dataaccess.DataAccessor[*model.Tenant]{},
		accounts: accounts,
		audit:    accounts.audit,
	}
}

func (s *StaffService) Register(ctx context.Context, req iamdto.RegisterRequest) (iamdto.LoginResult, error) {
	email := strings.ToLower(strings.TrimSpace(req.Email))
	displayName := strings.TrimSpace(req.DisplayName)
	password := req.Password
	if displayName == "" || email == "" || password == "" {
		return iamdto.LoginResult{}, badRequest("displayName, email and password are required")
	}
	if utf8.RuneCountInString(password) < 8 {
		return iamdto.LoginResult{}, badRequest("password must contain at least 8 characters")
	}
	if _, err := s.users.First(ctx, dataaccess.Eq("email", email)); err == nil {
		return iamdto.LoginResult{}, badRequest("email already exists")
	} else if !errors.Is(err, dataaccess.ErrDataNotFound) {
		return iamdto.LoginResult{}, err
	}
	userKey, err := generateUserKey()
	if err != nil {
		return iamdto.LoginResult{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return iamdto.LoginResult{}, err
	}
	account, err := generateStaffAccount()
	if err != nil {
		return iamdto.LoginResult{}, err
	}
	owner := &model.User{
		UserKey:      userKey,
		Account:      account,
		Email:        email,
		DisplayName:  displayName,
		PasswordHash: string(hash),
		TenantID:     userKey,
		Role:         "owner",
		Status:       "active",
	}
	tenantName := strings.TrimSpace(req.TenantName)
	if tenantName == "" {
		tenantName = displayName + " 的工作区"
	}
	tenant := &model.Tenant{
		TenantKey: userKey,
		Name:      tenantName,
		OwnerKey:  userKey,
		Status:    "active",
	}
	if err := dataaccess.Transaction(ctx, damodel.DBTypeRelation, damodel.DbSource("main"), func(tx context.Context) error {
		if err := s.users.Create(tx, owner); err != nil {
			return err
		}
		return s.tenants.Create(tx, tenant)
	}); err != nil {
		return iamdto.LoginResult{}, err
	}
	return s.Login(ctx, iamdto.LoginRequest{Identifier: email, Password: password}, iamdto.SessionContext{})
}

func (s *StaffService) Login(ctx context.Context, req iamdto.LoginRequest, sessionContext iamdto.SessionContext) (iamdto.LoginResult, error) {
	identifier := strings.ToLower(strings.TrimSpace(req.Identifier))
	if identifier == "" || req.Password == "" {
		return iamdto.LoginResult{}, badRequest("identifier and password are required")
	}
	if utf8.RuneCountInString(req.Password) < 8 {
		return iamdto.LoginResult{}, unauthorized("invalid identifier or password")
	}
	user, err := s.findUser(ctx, identifier)
	if err != nil {
		return iamdto.LoginResult{}, unauthorized("invalid identifier or password")
	}
	if user.Status != "active" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		return iamdto.LoginResult{}, unauthorized("invalid identifier or password")
	}
	now := time.Now()
	expiresAt := now.Add(sessionTTL)
	secret, err := generateToken("csim")
	if err != nil {
		return iamdto.LoginResult{}, err
	}
	tokenKey, err := generateTokenKey()
	if err != nil {
		return iamdto.LoginResult{}, err
	}
	sessionScopes, err := s.accounts.RoleScopes(ctx, user.TenantID, user.Role)
	if err != nil {
		return iamdto.LoginResult{}, err
	}
	session := &model.AccountToken{
		TokenKey:   tokenKey,
		AccountKey: user.UserKey,
		Name:       user.DisplayName,
		TenantID:   user.TenantID,
		Kind:       iamprincipal.KindHuman,
		ScopesJSON: encodeScopes(sessionScopes),
		TokenHash:  hashToken(secret),
		Device:     sessionContext.Device,
		IP:         sessionContext.IP,
		Status:     "active",
		ExpiresAt:  &expiresAt,
	}
	if err := s.accounts.accessor.Create(ctx, session); err != nil {
		return iamdto.LoginResult{}, err
	}
	user.LastLoginAt = &now
	if err := s.users.Update(ctx, user); err != nil {
		return iamdto.LoginResult{}, err
	}
	s.recordAudit(ctx, iamprincipal.Principal{
		AccountKey: user.UserKey,
		Name:       user.DisplayName,
		TenantID:   user.TenantID,
		Kind:       iamprincipal.KindHuman,
	}, "iam.auth", "login", user.UserKey, "success")
	return iamdto.LoginResult{User: staffInfo(user, session), Token: iamdto.Token{Token: secret, TokenKey: tokenKey, ExpiresAt: &expiresAt}}, nil
}

func (s *StaffService) Logout(ctx context.Context, principal iamprincipal.Principal) error {
	if principal.Kind != iamprincipal.KindHuman || principal.TokenKey == "" {
		return forbidden("only staff sessions can be logged out")
	}
	session, err := s.accounts.accessor.First(ctx, dataaccess.Eq("token_key", principal.TokenKey), dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return err
	}
	if session.AccountKey != principal.AccountKey || session.Kind != iamprincipal.KindHuman {
		return forbidden("session does not belong to this account")
	}
	now := time.Now()
	session.Status = "revoked"
	session.RevokedAt = &now
	if err := s.accounts.accessor.Update(ctx, session); err != nil {
		return err
	}
	s.recordAudit(ctx, principal, "iam.auth", "logout", principal.AccountKey, "success")
	return nil
}

func (s *StaffService) ChangeOwnPassword(ctx context.Context, principal iamprincipal.Principal, req iamdto.ChangeOwnPasswordRequest) error {
	if principal.Kind != iamprincipal.KindHuman {
		return forbidden("only staff can change password")
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return badRequest("currentPassword and newPassword are required")
	}
	if utf8.RuneCountInString(req.NewPassword) < 8 {
		return badRequest("new password must be at least 8 characters")
	}
	if req.NewPassword == req.CurrentPassword {
		return badRequest("new password must be different")
	}
	user, err := s.users.First(ctx, dataaccess.Eq("user_key", principal.AccountKey), dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return notFound("staff not found")
	}
	if user.Status != "active" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)) != nil {
		return unauthorized("current password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	if err := s.users.Update(ctx, user); err != nil {
		return err
	}
	if err := s.revokeSessions(ctx, user); err != nil {
		return err
	}
	s.recordAudit(ctx, principal, "iam.auth", "change_password", user.UserKey, "success")
	return nil
}

func (s *StaffService) List(ctx context.Context, principal iamprincipal.Principal) ([]iamdto.User, error) {
	if err := s.requireUserManager(principal); err != nil {
		return nil, err
	}
	items, err := s.users.List(ctx, dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return nil, err
	}
	sessions, err := s.accounts.accessor.List(
		ctx,
		dataaccess.Eq("tenant_id", principal.TenantID),
		dataaccess.Eq("kind", iamprincipal.KindHuman),
		dataaccess.Eq("status", "active"),
	)
	if err != nil {
		return nil, err
	}
	activeSessions := make(map[string]*model.AccountToken, len(sessions))
	for _, session := range sessions {
		if session.ExpiresAt != nil && session.ExpiresAt.Before(time.Now()) {
			continue
		}
		if current, exists := activeSessions[session.AccountKey]; !exists || (current.LastUsedAt == nil || (session.LastUsedAt != nil && session.LastUsedAt.After(*current.LastUsedAt))) {
			activeSessions[session.AccountKey] = session
		}
	}
	result := make([]iamdto.User, 0, len(items))
	for _, item := range items {
		result = append(result, staffInfo(item, activeSessions[item.UserKey]))
	}
	return result, nil
}

func (s *StaffService) Create(ctx context.Context, principal iamprincipal.Principal, req iamdto.CreateStaffRequest) (iamdto.CreateStaffResult, error) {
	if err := s.requireUserManager(principal); err != nil {
		return iamdto.CreateStaffResult{}, err
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" || email == "" {
		return iamdto.CreateStaffResult{}, badRequest("displayName and email are required")
	}
	if err := s.validateRole(ctx, principal.TenantID, req.Role); err != nil {
		return iamdto.CreateStaffResult{}, badRequest("unsupported staff role")
	}
	if _, err := s.users.First(ctx, dataaccess.Eq("email", email)); err == nil {
		return iamdto.CreateStaffResult{}, badRequest("staff email already exists")
	} else if !errors.Is(err, dataaccess.ErrDataNotFound) {
		return iamdto.CreateStaffResult{}, err
	}
	password, err := generateToken("pwd")
	if err != nil {
		return iamdto.CreateStaffResult{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return iamdto.CreateStaffResult{}, err
	}
	userKey, err := generateUserKey()
	if err != nil {
		return iamdto.CreateStaffResult{}, err
	}
	account, err := generateStaffAccount()
	if err != nil {
		return iamdto.CreateStaffResult{}, err
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
		return iamdto.CreateStaffResult{}, err
	}
	s.recordAudit(ctx, principal, "iam.staff", "create", item.UserKey, "success")
	return iamdto.CreateStaffResult{User: staffInfo(item, nil), InitialPassword: password}, nil
}

func (s *StaffService) SetRole(ctx context.Context, principal iamprincipal.Principal, userKey, role string) (iamdto.User, error) {
	item, err := s.accessStaff(ctx, principal, userKey)
	if err != nil {
		return iamdto.User{}, err
	}
	if err := s.validateRole(ctx, principal.TenantID, role); err != nil {
		return iamdto.User{}, badRequest("unsupported staff role")
	}
	item.Role = role
	if err := s.users.Update(ctx, item); err != nil {
		return iamdto.User{}, err
	}
	s.recordAudit(ctx, principal, "iam.staff", "set_role", item.UserKey, "success")
	return staffInfo(item, nil), nil
}

func (s *StaffService) Update(ctx context.Context, principal iamprincipal.Principal, userKey string, req iamdto.UpdateStaffRequest) (iamdto.User, error) {
	item, err := s.accessStaff(ctx, principal, userKey)
	if err != nil {
		return iamdto.User{}, err
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	displayName := strings.TrimSpace(req.DisplayName)
	if displayName == "" || email == "" {
		return iamdto.User{}, badRequest("displayName and email are required")
	}
	if err := s.validateRole(ctx, principal.TenantID, req.Role); err != nil {
		return iamdto.User{}, badRequest("unsupported staff role")
	}
	existing, err := s.users.First(ctx, dataaccess.Eq("email", email))
	if err == nil && existing.UserKey != userKey {
		return iamdto.User{}, badRequest("staff email already exists")
	}
	if err != nil && !errors.Is(err, dataaccess.ErrDataNotFound) {
		return iamdto.User{}, err
	}
	item.DisplayName = displayName
	item.Email = email
	item.Role = req.Role
	if err := s.users.Update(ctx, item); err != nil {
		return iamdto.User{}, err
	}
	s.recordAudit(ctx, principal, "iam.staff", "update", item.UserKey, "success")
	return staffInfo(item, nil), nil
}

func (s *StaffService) validateRole(ctx context.Context, tenantID, role string) error {
	if isBuiltinRole(role) {
		return nil
	}
	_, err := s.roles.First(ctx, dataaccess.Eq("tenant_id", tenantID), dataaccess.Eq("role_key", role))
	if err != nil {
		return err
	}
	return nil
}

func (s *StaffService) SetStatus(ctx context.Context, principal iamprincipal.Principal, userKey, status string) (iamdto.User, error) {
	item, err := s.accessStaff(ctx, principal, userKey)
	if err != nil {
		return iamdto.User{}, err
	}
	if status != "active" && status != "disabled" {
		return iamdto.User{}, badRequest("unsupported staff status")
	}
	item.Status = status
	if err := s.users.Update(ctx, item); err != nil {
		return iamdto.User{}, err
	}
	s.recordAudit(ctx, principal, "iam.staff", "set_status", item.UserKey, "success")
	if status == "disabled" {
		if err := s.revokeSessions(ctx, item); err != nil {
			return iamdto.User{}, err
		}
	}
	return staffInfo(item, nil), nil
}

func (s *StaffService) ResetPassword(ctx context.Context, principal iamprincipal.Principal, userKey string) (iamdto.CreateStaffResult, error) {
	item, err := s.accessStaff(ctx, principal, userKey)
	if err != nil {
		return iamdto.CreateStaffResult{}, err
	}
	password, err := generateToken("pwd")
	if err != nil {
		return iamdto.CreateStaffResult{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return iamdto.CreateStaffResult{}, err
	}
	item.PasswordHash = string(hash)
	if err := s.users.Update(ctx, item); err != nil {
		return iamdto.CreateStaffResult{}, err
	}
	if err := s.revokeSessions(ctx, item); err != nil {
		return iamdto.CreateStaffResult{}, err
	}
	s.recordAudit(ctx, principal, "iam.staff", "reset_password", item.UserKey, "success")
	return iamdto.CreateStaffResult{User: staffInfo(item, nil), InitialPassword: password}, nil
}

func (s *StaffService) ForceLogout(ctx context.Context, principal iamprincipal.Principal, userKey string) error {
	item, err := s.accessStaff(ctx, principal, userKey)
	if err != nil {
		return err
	}
	if err := s.revokeSessions(ctx, item); err != nil {
		return err
	}
	s.recordAudit(ctx, principal, "iam.staff", "force_logout", item.UserKey, "success")
	return nil
}

func (s *StaffService) recordAudit(ctx context.Context, principal iamprincipal.Principal, event, action, object, result string) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, principal, iamdto.AuditEntry{Event: event, Action: action, Object: object, Result: result})
}

func (s *StaffService) accessStaff(ctx context.Context, principal iamprincipal.Principal, userKey string) (*model.User, error) {
	if err := s.requireUserManager(principal); err != nil {
		return nil, err
	}
	if userKey == "" {
		return nil, badRequest("userKey is required")
	}
	if userKey == principal.AccountKey {
		return nil, forbidden("cannot manage current account")
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
	sessions, err := s.accounts.accessor.List(ctx, dataaccess.Eq("account_key", user.UserKey), dataaccess.Eq("tenant_id", user.TenantID), dataaccess.Eq("kind", iamprincipal.KindHuman))
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

func (s *StaffService) requireUserManager(principal iamprincipal.Principal) error {
	if principal.Kind != iamprincipal.KindHuman {
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
	return s.users.First(ctx, dataaccess.Eq("account", identifier))
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

func builtinRoleScopes(role string) []string {
	switch role {
	case "owner":
		return []string{ScopeAll, ScopeGatewayAdmin, ScopeIAMTokenManage, ScopeIAMUserManage, ScopeConversationRead, ScopeConversationManage, ScopeMessageSendAgent, ScopeAuditRead}
	case "admin":
		return []string{ScopeGatewayAdmin, ScopeIAMTokenManage, ScopeIAMUserManage, ScopeIAMRoleManage, ScopeChannelAccess, ScopeRelayAccess, ScopeConversationRead, ScopeConversationManage, ScopeMessageSendAgent, ScopeAuditRead}
	case "supervisor":
		return []string{ScopeGatewayAdmin, ScopeRelayAccess, ScopeConversationRead, ScopeConversationManage, ScopeMessageSendAgent, ScopeAuditRead}
	case "auditor":
		return []string{ScopeAuditRead}
	case "developer":
		return []string{ScopeChannelAccess, ScopeRelayAccess, ScopeConversationRead, ScopeAuditRead}
	default:
		return nil
	}
}

func staffInfo(user *model.User, session *model.AccountToken) iamdto.User {
	result := iamdto.User{
		UserKey:     user.UserKey,
		Account:     user.Account,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		TenantID:    user.TenantID,
		Role:        user.Role,
		Status:      user.Status,
		LastLoginAt: user.LastLoginAt,
	}
	if session == nil {
		return result
	}
	result.Online = true
	result.Device = session.Device
	result.IP = session.IP
	result.LastActiveAt = session.LastUsedAt
	if result.LastActiveAt == nil {
		createAt := session.CreateAt
		result.LastActiveAt = &createAt
	}
	return result
}
