package service

import (
	"context"
	"crypto/rand"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	"encoding/hex"
	"errors"
	"slices"
	"strings"

	iamdto "cs-im-server/internal/module/iam/dto"
	"cs-im-server/internal/module/iam/internal/model"

	"vane-common/pkg/dataaccess"
)

var supportedScopes = map[string]struct{}{
	ScopeAll:                {},
	ScopeGatewayAdmin:       {},
	ScopeIAMTokenManage:     {},
	ScopeIAMUserManage:      {},
	ScopeIAMRoleManage:      {},
	ScopeChannelAccess:      {},
	ScopeRelayAccess:        {},
	ScopeConversationRead:   {},
	ScopeConversationManage: {},
	ScopeMessageSendAgent:   {},
	ScopeAuditRead:          {},
}

type RoleService struct {
	roles dataaccess.DataAccessor[*model.Role]
	users dataaccess.DataAccessor[*model.User]
	audit *AuditService
}

func NewRoleService(audit *AuditService) *RoleService {
	return &RoleService{audit: audit}
}

func (s *RoleService) List(ctx context.Context, principal iamprincipal.Principal) ([]iamdto.RoleInfo, error) {
	if err := requireRoleManager(principal); err != nil {
		return nil, err
	}
	records, err := s.roles.List(ctx, dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return nil, err
	}
	users, err := s.users.List(ctx, dataaccess.Eq("tenant_id", principal.TenantID))
	if err != nil {
		return nil, err
	}
	members := make(map[string]int64, len(users))
	for _, user := range users {
		members[user.Role]++
	}
	recordsByKey := make(map[string]*model.Role, len(records))
	for _, item := range records {
		recordsByKey[item.RoleKey] = item
	}
	result := make([]iamdto.RoleInfo, 0, len(builtinRoles())+len(records))
	for _, builtin := range builtinRoles() {
		item, exists := recordsByKey[builtin.RoleKey]
		if exists {
			result = append(result, s.roleInfo(item, members[builtin.RoleKey]))
			delete(recordsByKey, builtin.RoleKey)
			continue
		}
		result = append(result, builtin)
	}
	for _, item := range records {
		if _, builtin := recordsByKey[item.RoleKey]; !builtin {
			continue
		}
		result = append(result, s.roleInfo(item, members[item.RoleKey]))
	}
	return result, nil
}

func (s *RoleService) Create(ctx context.Context, principal iamprincipal.Principal, req iamdto.CreateRoleRequest) (iamdto.RoleInfo, error) {
	if err := requireRoleManager(principal); err != nil {
		return iamdto.RoleInfo{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return iamdto.RoleInfo{}, badRequest("role name is required")
	}
	scopes, err := normalizeScopes(principal, req.Scopes)
	if err != nil {
		return iamdto.RoleInfo{}, err
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "disabled" {
		return iamdto.RoleInfo{}, badRequest("unsupported role status")
	}
	roleKey, err := generateRoleKey()
	if err != nil {
		return iamdto.RoleInfo{}, err
	}
	item := &model.Role{
		TenantID:    principal.TenantID,
		RoleKey:     roleKey,
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		Kind:        "custom",
		Status:      status,
		ScopesJSON:  encodeScopes(scopes),
	}
	if err := s.roles.Create(ctx, item); err != nil {
		return iamdto.RoleInfo{}, err
	}
	s.recordAudit(ctx, principal, "iam.role", "create", item.RoleKey, "success")
	return s.roleInfo(item, 0), nil
}

func (s *RoleService) Update(ctx context.Context, principal iamprincipal.Principal, roleKey string, req iamdto.UpdateRoleRequest) (iamdto.RoleInfo, error) {
	if err := requireRoleManager(principal); err != nil {
		return iamdto.RoleInfo{}, err
	}
	if roleKey == "owner" {
		return iamdto.RoleInfo{}, forbidden("owner role cannot be edited")
	}
	scopes, err := normalizeScopes(principal, req.Scopes)
	if err != nil {
		return iamdto.RoleInfo{}, err
	}
	status := req.Status
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "disabled" {
		return iamdto.RoleInfo{}, badRequest("unsupported role status")
	}
	item, err := s.roles.First(ctx, dataaccess.Eq("tenant_id", principal.TenantID), dataaccess.Eq("role_key", roleKey))
	if errors.Is(err, dataaccess.ErrDataNotFound) {
		builtin, found := builtinRole(roleKey)
		if !found {
			return iamdto.RoleInfo{}, dataaccess.ErrDataNotFound
		}
		item = &model.Role{
			TenantID:    principal.TenantID,
			RoleKey:     builtin.RoleKey,
			Name:        builtin.Name,
			Description: builtin.Description,
			Kind:        "system",
			Status:      builtin.Status,
			ScopesJSON:  encodeScopes(builtin.Scopes),
		}
		if err := s.roles.Create(ctx, item); err != nil {
			return iamdto.RoleInfo{}, err
		}
	} else if err != nil {
		return iamdto.RoleInfo{}, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return iamdto.RoleInfo{}, badRequest("role name is required")
	}
	item.Name = name
	item.Description = strings.TrimSpace(req.Description)
	item.Status = status
	item.ScopesJSON = encodeScopes(scopes)
	if item.GetID().Val() != nil && item.GetID().Val() != int64(0) {
		if err := s.roles.Update(ctx, item); err != nil {
			return iamdto.RoleInfo{}, err
		}
	}
	users, err := s.users.List(ctx, dataaccess.Eq("tenant_id", principal.TenantID), dataaccess.Eq("role", roleKey))
	if err != nil {
		return iamdto.RoleInfo{}, err
	}
	s.recordAudit(ctx, principal, "iam.role", "update", item.RoleKey, "success")
	return s.roleInfo(item, int64(len(users))), nil
}

func (s *RoleService) Delete(ctx context.Context, principal iamprincipal.Principal, roleKey string) error {
	if err := requireRoleManager(principal); err != nil {
		return err
	}
	if _, builtin := builtinRole(roleKey); builtin || roleKey == "owner" {
		return forbidden("system role cannot be deleted")
	}
	users, err := s.users.List(ctx, dataaccess.Eq("tenant_id", principal.TenantID), dataaccess.Eq("role", roleKey))
	if err != nil {
		return err
	}
	if len(users) > 0 {
		return badRequest("role is still assigned to staff")
	}
	item, err := s.roles.First(ctx, dataaccess.Eq("tenant_id", principal.TenantID), dataaccess.Eq("role_key", roleKey))
	if err != nil {
		return err
	}
	s.recordAudit(ctx, principal, "iam.role", "delete", item.RoleKey, "success")
	return s.roles.Delete(ctx, item)
}

func (s *RoleService) recordAudit(ctx context.Context, principal iamprincipal.Principal, event, action, object, result string) {
	if s.audit == nil {
		return
	}
	s.audit.Record(ctx, principal, iamdto.AuditEntry{Event: event, Action: action, Object: object, Result: result})
}

func requireRoleManager(principal iamprincipal.Principal) error {
	if principal.Kind != iamprincipal.KindHuman {
		return forbidden("only staff can manage roles")
	}
	if !principal.HasScope(ScopeIAMRoleManage) {
		return forbidden("principal cannot manage roles")
	}
	return nil
}

func normalizeScopes(principal iamprincipal.Principal, scopes []string) ([]string, error) {
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, ok := supportedScopes[scope]; !ok {
			return nil, badRequest("unsupported permission: %s", scope)
		}
		if scope == ScopeAll && !principal.HasScope(ScopeAll) {
			return nil, forbidden("principal cannot grant all permissions")
		}
		if !slices.Contains(result, scope) {
			result = append(result, scope)
		}
	}
	slices.Sort(result)
	return result, nil
}

func builtinRoles() []iamdto.RoleInfo {
	return []iamdto.RoleInfo{
		{
			RoleKey: "admin", Name: "管理员", Description: "拥有租户配置、员工、渠道、会话和审计的完整管理能力。", Kind: "system", Status: "active",
			Scopes: builtinRoleScopes("admin"),
		},
		{
			RoleKey: "supervisor", Name: "主管", Description: "查看租户会话，分配和接管会话，查看运营状态。", Kind: "system", Status: "active",
			Scopes: builtinRoleScopes("supervisor"),
		},
		{RoleKey: "agent", Name: "客服", Description: "处理分配给自己的会话并发送人工回复。", Kind: "system", Status: "active", Scopes: builtinRoleScopes("agent")},
		{RoleKey: "auditor", Name: "审计员", Description: "只读查看审计日志和敏感操作记录。", Kind: "system", Status: "active", Scopes: builtinRoleScopes("auditor")},
		{RoleKey: "developer", Name: "开发者", Description: "管理渠道接入凭证并排查消息链路。", Kind: "system", Status: "active", Scopes: builtinRoleScopes("developer")},
	}
}

func builtinRole(roleKey string) (iamdto.RoleInfo, bool) {
	for _, item := range builtinRoles() {
		if item.RoleKey == roleKey {
			return item, true
		}
	}
	return iamdto.RoleInfo{}, false
}

func isBuiltinRole(roleKey string) bool {
	_, ok := builtinRole(roleKey)
	return ok
}

func (s *RoleService) roleInfo(item *model.Role, members int64) iamdto.RoleInfo {
	return iamdto.RoleInfo{
		RoleKey: item.RoleKey, Name: item.Name, Description: item.Description, Kind: item.Kind, Status: item.Status, Scopes: decodeScopesMust(item.ScopesJSON), Members: members,
	}
}

func generateRoleKey() (string, error) {
	key := make([]byte, 8)
	if _, err := rand.Read(key); err != nil {
		return "", err
	}
	return "custom_" + hex.EncodeToString(key), nil
}
