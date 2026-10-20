package service

import (
	"context"
	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"testing"

	gatewaymodel "cs-im-server/internal/module/gateway/internal/model"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/sqlitedb"
)

func setupAppService(t *testing.T) *AppService {
	t.Helper()

	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	if err := db.AutoMigrate(&gatewaymodel.App{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}

	service := NewAppService(NewRealtimeService())
	service.accessor = dataaccess.DataAccessor[*App]{}
	return service
}

func businessPrincipal(tenantID string) Principal {
	return Principal{
		AccountKey: "account_" + tenantID,
		TenantID:   tenantID,
		Kind:       iampkg.PrincipalKindBusiness,
		Scopes:     []string{iampkg.ScopeChannelAccess},
	}
}

func staffPrincipal(tenantID string) Principal {
	return Principal{
		AccountKey: "staff_" + tenantID,
		TenantID:   tenantID,
		Kind:       iampkg.PrincipalKindHuman,
		Scopes:     []string{iampkg.ScopeGatewayAdmin},
	}
}

func registerApp(t *testing.T, service *AppService, tenantID, serviceKey, name string) App {
	t.Helper()
	app, err := service.Register(context.Background(), staffPrincipal(tenantID), gatewaydto.RegisterAppRequest{
		ServiceKey:  serviceKey,
		DisplayName: name,
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	return app
}

func TestAppServiceTenantBoundary(t *testing.T) {
	service := setupAppService(t)
	tenant1App := registerApp(t, service, "tenant_1", "web", "官网客服")
	tenant2App := registerApp(t, service, "tenant_2", "web", "小程序客服")

	principal1 := staffPrincipal("tenant_1")
	principal2 := staffPrincipal("tenant_2")

	apps1 := service.List(context.Background(), principal1)
	if len(apps1) != 1 || apps1[0].AppKey != tenant1App.AppKey {
		t.Fatalf("List() for tenant 1 = %+v", apps1)
	}
	apps2 := service.List(context.Background(), principal2)
	if len(apps2) != 1 || apps2[0].AppKey != tenant2App.AppKey {
		t.Fatalf("List() for tenant 2 = %+v", apps2)
	}

	if _, err := service.GetAuthorized(context.Background(), principal1, tenant2App.AppKey); err == nil {
		t.Fatal("GetAuthorized() allowed other tenant app")
	}
	if _, err := service.Heartbeat(context.Background(), principal1, gatewaydto.HeartbeatRequest{AppID: tenant2App.AppKey}); err == nil {
		t.Fatal("Heartbeat() allowed other tenant app")
	}
	if _, err := service.SetStatus(context.Background(), principal1, tenant2App.AppKey, false); err == nil {
		t.Fatal("SetStatus() allowed other tenant app")
	}
}

func TestAppServiceRequiresChannelScope(t *testing.T) {
	service := setupAppService(t)
	app := registerApp(t, service, "tenant_1", "web", "官网客服")

	missingScope := Principal{
		AccountKey: "account_tenant_1",
		TenantID:   "tenant_1",
		Kind:       iampkg.PrincipalKindBusiness,
	}
	if err := service.AuthorizeAccount(app.AppKey, missingScope); err == nil {
		t.Fatal("AuthorizeAccount() allowed principal without channel scope")
	}

	relay := Principal{
		AccountKey: "relay_tenant_1",
		TenantID:   "tenant_1",
		Kind:       iampkg.PrincipalKindRelay,
		Scopes:     []string{iampkg.ScopeAll},
	}
	if err := service.AuthorizeAccount(app.AppKey, relay); err == nil {
		t.Fatal("AuthorizeAccount() allowed relay principal")
	}
}

func TestAppServiceMarkConnected(t *testing.T) {
	service := setupAppService(t)
	app := registerApp(t, service, "tenant_1", "frontend", "Demo Frontend")

	if app.LastHeartbeatAt != nil || app.LeaseExpiresAt != nil {
		t.Fatal("registered app should start without a live lease")
	}
	service.MarkConnected(app.AppKey)
	updated, err := service.Get(context.Background(), app.AppKey)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if updated.LastHeartbeatAt == nil || updated.LeaseExpiresAt == nil {
		t.Fatalf("MarkConnected() did not set lease: %+v", updated)
	}
}
