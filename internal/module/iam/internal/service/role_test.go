package service_test

import (
	"context"
	iamdto "cs-im-server/internal/module/iam/dto"
	"testing"

	iamservice "cs-im-server/internal/module/iam/internal/service"
)

func TestRolePermissionsAffectAuthentication(t *testing.T) {
	staffService := setupStaffService(t)
	login, err := staffService.Login(context.Background(), iamdto.LoginRequest{
		Identifier: "admin@cs-im.local",
		Password:   "admin12345",
	}, iamdto.SessionContext{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	principal, err := staffService.Accounts().Authenticate(context.Background(), login.Token.Token)
	if err != nil || !principal.HasScope(iamservice.ScopeIAMRoleManage) {
		t.Fatalf("Authenticate() = %+v, error = %v", principal, err)
	}

	roleService := iamservice.NewRoleService(iamservice.NewAuditService())
	updated, err := roleService.Update(context.Background(), principal, "admin", iamdto.UpdateRoleRequest{
		Name:        "管理员",
		Description: "受限管理员",
		Status:      "active",
		Scopes:      []string{"conversation:read"},
	})
	if err != nil || len(updated.Scopes) != 1 || updated.Scopes[0] != "conversation:read" {
		t.Fatalf("Update() = %+v, error = %v", updated, err)
	}
	refreshed, err := staffService.Accounts().Authenticate(context.Background(), login.Token.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if refreshed.HasScope(iamservice.ScopeIAMRoleManage) || !refreshed.HasScope("conversation:read") {
		t.Fatalf("Authenticate() = %+v", refreshed)
	}

	created, err := roleService.Create(context.Background(), principal, iamdto.CreateRoleRequest{
		Name:   "运营",
		Status: "active",
		Scopes: []string{"conversation:manage"},
	})
	if err != nil || created.Kind != "custom" {
		t.Fatalf("Create() = %+v, error = %v", created, err)
	}
	member, err := staffService.Create(context.Background(), principal, iamdto.CreateStaffRequest{
		DisplayName: "运营成员",
		Email:       "custom-role@example.com",
		Role:        created.RoleKey,
	})
	if err != nil || member.User.Role != created.RoleKey {
		t.Fatalf("Create() = %+v, error = %v", member.User, err)
	}
	memberLogin, err := staffService.Login(context.Background(), iamdto.LoginRequest{
		Identifier: "custom-role@example.com",
		Password:   member.InitialPassword,
	}, iamdto.SessionContext{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	custom, err := staffService.Accounts().Authenticate(context.Background(), memberLogin.Token.Token)
	if err != nil || custom.HasScope(iamservice.ScopeIAMRoleManage) || !custom.HasScope("conversation:manage") {
		t.Fatalf("Authenticate() = %+v, error = %v", custom, err)
	}
}
