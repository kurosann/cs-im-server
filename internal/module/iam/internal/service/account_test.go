package service_test

import (
	"context"
	iamdto "cs-im-server/internal/module/iam/dto"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	"testing"
	"time"

	iammodel "cs-im-server/internal/module/iam/internal/model"
	iamservice "cs-im-server/internal/module/iam/internal/service"

	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/sqlitedb"
)

func setupAccountService(t *testing.T) *iamservice.AccountService {
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
	return iamservice.NewAccountService("bootstrap-token", "tenant_1", "relay-token")
}

func bootstrapPrincipal() iamprincipal.Principal {
	return iamprincipal.Principal{
		AccountKey: "dev-account",
		TenantID:   "tenant_1",
		Kind:       iamprincipal.KindBusiness,
		Scopes:     []string{iamservice.ScopeAll},
	}
}

func issueToken(t *testing.T, service *iamservice.AccountService, principal iamprincipal.Principal, req iamdto.IssueAccountTokenRequest) iamdto.IssueAccountTokenResult {
	t.Helper()
	result, err := service.Issue(context.Background(), principal, req)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if result.Secret == "" || result.Token.TokenKey == "" {
		t.Fatalf("Issue() = %+v", result)
	}
	return result
}

func TestAccountTokenLifecycle(t *testing.T) {
	service := setupAccountService(t)
	issued := issueToken(t, service, bootstrapPrincipal(), iamdto.IssueAccountTokenRequest{
		AccountKey: "app-account",
		Name:       "app token",
	})

	principal, err := service.Authenticate(context.Background(), issued.Secret)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if principal.TenantID != "tenant_1" || principal.Kind != iamprincipal.KindBusiness || !principal.HasScope(iamservice.ScopeChannelAccess) {
		t.Fatalf("Authenticate() = %+v", principal)
	}

	disabled, err := service.SetStatus(context.Background(), bootstrapPrincipal(), issued.Token.TokenKey, "disabled")
	if err != nil || disabled.Status != "disabled" {
		t.Fatalf("SetStatus() = %+v, error = %v", disabled, err)
	}
	if _, err := service.Authenticate(context.Background(), issued.Secret); err == nil {
		t.Fatal("Authenticate() did not reject disabled token")
	}

	if _, err := service.SetStatus(context.Background(), bootstrapPrincipal(), issued.Token.TokenKey, "active"); err != nil {
		t.Fatalf("Enable() error = %v", err)
	}
	if _, err := service.Authenticate(context.Background(), issued.Secret); err != nil {
		t.Fatalf("Authenticate() after enable error = %v", err)
	}

	rotated, err := service.Rotate(context.Background(), bootstrapPrincipal(), issued.Token.TokenKey)
	if err != nil {
		t.Fatalf("Rotate() error = %v", err)
	}
	if _, err := service.Authenticate(context.Background(), issued.Secret); err == nil {
		t.Fatal("Authenticate() accepted revoked token")
	}
	if _, err := service.Authenticate(context.Background(), rotated.Secret); err != nil {
		t.Fatalf("Authenticate() rotated token error = %v", err)
	}
}

func TestAccountTokenExpiry(t *testing.T) {
	service := setupAccountService(t)
	expiresAt := time.Now().Add(-time.Minute)
	issued := issueToken(t, service, bootstrapPrincipal(), iamdto.IssueAccountTokenRequest{
		AccountKey: "app-account",
		ExpiresAt:  &expiresAt,
	})
	if _, err := service.Authenticate(context.Background(), issued.Secret); err == nil {
		t.Fatal("Authenticate() accepted expired token")
	}
}

func TestAccountTokenPermissionBoundary(t *testing.T) {
	service := setupAccountService(t)
	issued := issueToken(t, service, bootstrapPrincipal(), iamdto.IssueAccountTokenRequest{
		AccountKey: "app-account",
	})
	principal, err := service.Authenticate(context.Background(), issued.Secret)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if _, err := service.Issue(context.Background(), principal, iamdto.IssueAccountTokenRequest{AccountKey: "app-account"}); err == nil {
		t.Fatal("Issue() allowed token without management scope")
	}

	manager := issueToken(t, service, bootstrapPrincipal(), iamdto.IssueAccountTokenRequest{
		AccountKey: "manager-account",
		Scopes:     []string{iamservice.ScopeIAMTokenManage, iamservice.ScopeRelayAccess},
	})
	managerPrincipal, err := service.Authenticate(context.Background(), manager.Secret)
	if err != nil {
		t.Fatalf("Authenticate() manager error = %v", err)
	}
	if _, err := service.Issue(context.Background(), managerPrincipal, iamdto.IssueAccountTokenRequest{
		AccountKey: "manager-account",
		Kind:       iamprincipal.KindRelay,
		Scopes:     []string{iamservice.ScopeRelayAccess},
	}); err == nil {
		t.Fatal("Issue() allowed business token to issue relay token")
	}
}

func TestAccountTokenTenantBoundary(t *testing.T) {
	service := setupAccountService(t)
	issued := issueToken(t, service, bootstrapPrincipal(), iamdto.IssueAccountTokenRequest{
		AccountKey: "app-account",
	})
	principal, err := service.Authenticate(context.Background(), issued.Secret)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if _, err := service.Get(context.Background(), principal, issued.Token.TokenKey); err == nil {
		t.Fatal("Get() allowed same account without management scope")
	}

	bootstrap := issueToken(t, service, bootstrapPrincipal(), iamdto.IssueAccountTokenRequest{
		AccountKey: "other-account",
		TenantID:   "tenant_2",
	})
	other, err := service.Authenticate(context.Background(), bootstrap.Secret)
	if err != nil {
		t.Fatalf("Authenticate() other tenant error = %v", err)
	}
	if other.TenantID != "tenant_2" {
		t.Fatalf("Authenticate() = %+v", other)
	}
}
