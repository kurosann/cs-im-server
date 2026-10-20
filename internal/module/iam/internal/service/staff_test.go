package service_test

import (
	"context"
	iamdto "cs-im-server/internal/module/iam/dto"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	"testing"

	iammodel "cs-im-server/internal/module/iam/internal/model"
	iamservice "cs-im-server/internal/module/iam/internal/service"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/sqlitedb"
)

func setupStaffService(t *testing.T) *iamservice.StaffService {
	t.Helper()
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	if err := db.AutoMigrate(&iammodel.AccountToken{}, &iammodel.ChannelCredential{}, &iammodel.Tenant{}, &iammodel.Role{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	if err := db.AutoMigrate(&iammodel.User{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	accountService := iamservice.NewAccountService("bootstrap-token", "tenant_1", "relay-token")
	return iamservice.NewStaffService(accountService, "admin@cs-im.local", "admin12345", "tenant_1")
}

func TestStaffLoginAndLogout(t *testing.T) {
	service := setupStaffService(t)
	login, err := service.Login(context.Background(), iamdto.LoginRequest{
		Identifier: "admin@cs-im.local",
		Password:   "admin12345",
	}, iamdto.SessionContext{Device: "Web 工作台", IP: "192.0.2.1"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if login.Token.Token == "" || login.User.Role != "admin" {
		t.Fatalf("Login() = %+v", login)
	}
	principal, err := service.Accounts().Authenticate(context.Background(), login.Token.Token)
	if err != nil || principal.Kind != iamprincipal.KindHuman {
		t.Fatalf("Authenticate() = %+v, error = %v", principal, err)
	}
	if err := service.Logout(context.Background(), principal); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := service.Accounts().Authenticate(context.Background(), login.Token.Token); err == nil {
		t.Fatal("Authenticate() accepted logged-out token")
	}
}

func TestStaffPermissionsRefreshOnAuthenticate(t *testing.T) {
	service := setupStaffService(t)
	login, err := service.Login(context.Background(), iamdto.LoginRequest{
		Identifier: "admin@cs-im.local",
		Password:   "admin12345",
	}, iamdto.SessionContext{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	principal, err := service.Accounts().Authenticate(context.Background(), login.Token.Token)
	if err != nil || !principal.HasScope(iamservice.ScopeIAMTokenManage) {
		t.Fatalf("Authenticate() = %+v, error = %v", principal, err)
	}

	users := dataaccess.DataAccessor[*iammodel.User]{}
	user, err := users.First(context.Background(), dataaccess.Eq("user_key", login.User.UserKey))
	if err != nil {
		t.Fatalf("load staff: %v", err)
	}
	user.Role = "auditor"
	if err := users.Update(context.Background(), user); err != nil {
		t.Fatalf("update staff role: %v", err)
	}

	refreshed, err := service.Accounts().Authenticate(context.Background(), login.Token.Token)
	if err != nil {
		t.Fatalf("Authenticate() after role update error = %v", err)
	}
	if refreshed.HasScope(iamservice.ScopeIAMTokenManage) || !refreshed.HasScope("audit:read") {
		t.Fatalf("Authenticate() did not refresh permissions: %+v", refreshed)
	}
}

func TestStaffManagementUpdatesPermissionsImmediately(t *testing.T) {
	service := setupStaffService(t)
	admin, err := service.Login(context.Background(), iamdto.LoginRequest{
		Identifier: "admin@cs-im.local",
		Password:   "admin12345",
	}, iamdto.SessionContext{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	principal, err := service.Accounts().Authenticate(context.Background(), admin.Token.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	created, err := service.Create(context.Background(), principal, iamdto.CreateStaffRequest{
		DisplayName: "测试员工",
		Email:       "member@example.com",
		Role:        "developer",
	})
	if err != nil || created.InitialPassword == "" {
		t.Fatalf("Create() = %+v, error = %v", created, err)
	}
	staff, err := service.List(context.Background(), principal)
	if err != nil || len(staff) != 2 {
		t.Fatalf("List() = %+v, error = %v", staff, err)
	}
	updated, err := service.SetRole(context.Background(), principal, created.User.UserKey, "auditor")
	if err != nil || updated.Role != "auditor" {
		t.Fatalf("SetRole() = %+v, error = %v", updated, err)
	}
	if _, err := service.SetStatus(context.Background(), principal, created.User.UserKey, "disabled"); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}
	if _, err := service.Accounts().Authenticate(context.Background(), admin.Token.Token); err != nil {
		t.Fatalf("admin authentication should still work: %v", err)
	}
}

func TestStaffCannotManageSelf(t *testing.T) {
	service := setupStaffService(t)
	login, err := service.Login(context.Background(), iamdto.LoginRequest{
		Identifier: "admin@cs-im.local",
		Password:   "admin12345",
	}, iamdto.SessionContext{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	principal, err := service.Accounts().Authenticate(context.Background(), login.Token.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if _, err := service.SetRole(context.Background(), principal, principal.AccountKey, "agent"); err == nil {
		t.Fatal("SetRole() allowed self-management")
	}
	if _, err := service.SetStatus(context.Background(), principal, principal.AccountKey, "disabled"); err == nil {
		t.Fatal("SetStatus() allowed self-management")
	}
}

func TestStaffListShowsSessionAndForceLogout(t *testing.T) {
	service := setupStaffService(t)
	admin, err := service.Login(context.Background(), iamdto.LoginRequest{
		Identifier: "admin@cs-im.local",
		Password:   "admin12345",
	}, iamdto.SessionContext{Device: "Chrome", IP: "192.0.2.8"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	principal, err := service.Accounts().Authenticate(context.Background(), admin.Token.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	created, err := service.Create(context.Background(), principal, iamdto.CreateStaffRequest{
		DisplayName: "在线员工",
		Email:       "online@example.com",
		Role:        "agent",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	memberLogin, err := service.Login(context.Background(), iamdto.LoginRequest{
		Identifier: "online@example.com",
		Password:   created.InitialPassword,
	}, iamdto.SessionContext{Device: "Safari", IP: "192.0.2.9"})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	items, err := service.List(context.Background(), principal)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	var online bool
	for _, item := range items {
		if item.UserKey == created.User.UserKey {
			online = item.Online && item.Device == "Safari" && item.IP == "192.0.2.9"
		}
	}
	if !online {
		t.Fatalf("List() did not show active session: %+v", items)
	}
	if err := service.ForceLogout(context.Background(), principal, created.User.UserKey); err != nil {
		t.Fatalf("ForceLogout() error = %v", err)
	}
	if _, err := service.Accounts().Authenticate(context.Background(), memberLogin.Token.Token); err == nil {
		t.Fatal("forced session is still active")
	}
}

func TestStaffUpdateChangesProfileAndRole(t *testing.T) {
	service := setupStaffService(t)
	login, err := service.Login(context.Background(), iamdto.LoginRequest{
		Identifier: "admin@cs-im.local",
		Password:   "admin12345",
	}, iamdto.SessionContext{})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	principal, err := service.Accounts().Authenticate(context.Background(), login.Token.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	created, err := service.Create(context.Background(), principal, iamdto.CreateStaffRequest{
		DisplayName: "旧姓名",
		Email:       "old@example.com",
		Role:        "agent",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	updated, err := service.Update(context.Background(), principal, created.User.UserKey, iamdto.UpdateStaffRequest{
		DisplayName: "新姓名",
		Email:       "new@example.com",
		Role:        "auditor",
	})
	if err != nil || updated.DisplayName != "新姓名" || updated.Email != "new@example.com" || updated.Role != "auditor" {
		t.Fatalf("Update() = %+v, error = %v", updated, err)
	}
	if _, err := service.Update(context.Background(), principal, principal.AccountKey, iamdto.UpdateStaffRequest{
		DisplayName: "当前账号",
		Email:       "current@example.com",
		Role:        "agent",
	}); err == nil {
		t.Fatal("Update() allowed self-management")
	}
}

func TestRegisterCreatesTenantWithOwnerAccount(t *testing.T) {
	service := setupStaffService(t)
	login, err := service.Register(context.Background(), iamdto.RegisterRequest{
		DisplayName: "租户主账号",
		Email:       "owner@example.com",
		Password:    "password123",
		TenantName:  "测试租户",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if login.User.Role != "owner" || login.User.TenantID != login.User.UserKey {
		t.Fatalf("Register() = %+v", login.User)
	}
	principal, err := service.Accounts().Authenticate(context.Background(), login.Token.Token)
	if err != nil || !principal.HasScope(iamservice.ScopeAll) {
		t.Fatalf("Authenticate() = %+v, error = %v", principal, err)
	}
	tenants := dataaccess.DataAccessor[*iammodel.Tenant]{}
	tenant, err := tenants.First(context.Background(), dataaccess.Eq("tenant_key", login.User.UserKey))
	if err != nil || tenant.OwnerKey != login.User.UserKey || tenant.Name != "测试租户" {
		t.Fatalf("Tenant = %+v, error = %v", tenant, err)
	}

	staff, err := service.Create(context.Background(), principal, iamdto.CreateStaffRequest{
		DisplayName: "租户成员",
		Email:       "member-owner@example.com",
		Role:        "agent",
	})
	if err != nil || staff.User.TenantID != login.User.TenantID {
		t.Fatalf("Create() = %+v, error = %v", staff.User, err)
	}
}
