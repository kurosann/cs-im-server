package service_test

import (
	"context"
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
	if err := db.AutoMigrate(&iammodel.AccountToken{}, &iammodel.ChannelCredential{}); err != nil {
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
	login, err := service.Login(context.Background(), iamservice.LoginRequest{
		Identifier: "admin@cs-im.local",
		Password:   "admin12345",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if login.Token.Token == "" || login.User.Role != "admin" {
		t.Fatalf("Login() = %+v", login)
	}
	principal, err := service.Accounts().Authenticate(context.Background(), login.Token.Token)
	if err != nil || principal.Kind != iamservice.PrincipalKindHuman {
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
	login, err := service.Login(context.Background(), iamservice.LoginRequest{
		Identifier: "admin@cs-im.local",
		Password:   "admin12345",
	})
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
	admin, err := service.Login(context.Background(), iamservice.LoginRequest{
		Identifier: "admin@cs-im.local",
		Password:   "admin12345",
	})
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	principal, err := service.Accounts().Authenticate(context.Background(), admin.Token.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	created, err := service.Create(context.Background(), principal, iamservice.CreateStaffRequest{
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
