package service_test

import (
	"context"
	iamdto "cs-im-server/internal/module/iam/dto"
	iamprincipal "cs-im-server/internal/module/iam/pkg/principal"
	"testing"
	"time"

	iamservice "cs-im-server/internal/module/iam/internal/service"
)

func staffIAMPrincipal() iamprincipal.Principal {
	return iamprincipal.Principal{
		AccountKey: "staff_1",
		TenantID:   "tenant_1",
		Kind:       iamprincipal.KindHuman,
		Scopes:     []string{iamservice.ScopeIAMTokenManage},
	}
}

func TestChannelCredentialLifecycle(t *testing.T) {
	service := setupAccountService(t)
	issued, err := service.IssueChannelCredential(context.Background(), staffIAMPrincipal(), iamdto.IssueChannelCredentialRequest{
		AppID: "app_test", Name: "官网客服接入",
	})
	if err != nil {
		t.Fatalf("IssueChannelCredential() error = %v", err)
	}

	principal, err := service.Authenticate(context.Background(), issued.Secret)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if principal.AccountKey != "app_test" || principal.TenantID != "tenant_1" || principal.Kind != iamprincipal.KindBusiness || !principal.HasScope(iamservice.ScopeChannelAccess) {
		t.Fatalf("Authenticate() = %+v", principal)
	}

	disabled, err := service.SetChannelCredentialStatus(context.Background(), staffIAMPrincipal(), "app_test", "disabled")
	if err != nil || disabled.Status != "disabled" {
		t.Fatalf("SetChannelCredentialStatus() = %+v, error = %v", disabled, err)
	}
	if _, err := service.Authenticate(context.Background(), issued.Secret); err == nil {
		t.Fatal("Authenticate() accepted disabled credential")
	}

	if _, err := service.SetChannelCredentialStatus(context.Background(), staffIAMPrincipal(), "app_test", "active"); err != nil {
		t.Fatalf("Enable credential error = %v", err)
	}
	rotated, err := service.RotateChannelCredential(context.Background(), staffIAMPrincipal(), "app_test")
	if err != nil {
		t.Fatalf("RotateChannelCredential() error = %v", err)
	}
	if _, err := service.Authenticate(context.Background(), issued.Secret); err == nil {
		t.Fatal("Authenticate() accepted rotated credential")
	}
	if _, err := service.Authenticate(context.Background(), rotated.Secret); err != nil {
		t.Fatalf("Authenticate() rotated credential error = %v", err)
	}

	if err := service.DeleteChannelCredential(context.Background(), staffIAMPrincipal(), "app_test"); err != nil {
		t.Fatalf("DeleteChannelCredential() error = %v", err)
	}
	if _, err := service.Authenticate(context.Background(), rotated.Secret); err == nil {
		t.Fatal("Authenticate() accepted deleted credential")
	}
	credentials, err := service.ListChannelCredentials(context.Background(), staffIAMPrincipal())
	if err != nil || len(credentials) != 0 {
		t.Fatalf("ListChannelCredentials() = %+v, error = %v", credentials, err)
	}
	if appIDs, err := service.ExistingChannelAppIDs(context.Background(), staffIAMPrincipal()); err != nil || len(appIDs) != 0 {
		t.Fatalf("ExistingChannelAppIDs() = %+v, error = %v", appIDs, err)
	}
}

func TestChannelCredentialExpiry(t *testing.T) {
	service := setupAccountService(t)
	expiresAt := time.Now().Add(-time.Minute)
	issued, err := service.IssueChannelCredential(context.Background(), staffIAMPrincipal(), iamdto.IssueChannelCredentialRequest{
		AppID: "app_expired", Name: "过期渠道", ExpiresAt: &expiresAt,
	})
	if err != nil {
		t.Fatalf("IssueChannelCredential() error = %v", err)
	}
	if _, err := service.Authenticate(context.Background(), issued.Secret); err == nil {
		t.Fatal("Authenticate() accepted expired credential")
	}
}
