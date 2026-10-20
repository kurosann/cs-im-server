package service

import (
	"context"
	"errors"
	"testing"

	relaydto "cs-im-server/internal/module/relay/dto"

	"vane-common/pkg/dataaccess/store"
	"vane-common/pkg/sqlitedb"
)

func setupEmployees(t *testing.T) *RelayService {
	t.Helper()
	db, err := sqlitedb.NewSqliteDB(sqlitedb.Config{DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	store.RegisterRelationDB("main", store.RelationStore{DB: db.DB, Closer: db})
	if err := db.AutoMigrate(&Channel{}, &AIEmployee{}, &AIEmployeeBinding{}, &businessAppRow{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return NewRelayService(realtimePublisherStub{})
}

type realtimePublisherStub struct{}

func (realtimePublisherStub) Publish(string, string, any) {}

func TestAIEmployeeAccountCRUD(t *testing.T) {
	ctx := context.Background()
	service := setupEmployees(t)
	if err := service.apps.Create(ctx, &businessAppRow{
		AppKey: "app_sales", TenantID: "tenant-a", DisplayName: "售前业务", Kind: "WhatsApp",
	}); err != nil {
		t.Fatalf("create app: %v", err)
	}
	if err := service.apps.Create(ctx, &businessAppRow{
		AppKey: "app_other", TenantID: "tenant-b", DisplayName: "其他租户", Kind: "telegram",
	}); err != nil {
		t.Fatalf("create other app: %v", err)
	}
	channel, err := service.CreateChannel(ctx, relaydto.CreateChannelRequest{Name: "官网转发", Type: "web"})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}

	empty, err := service.ListEmployees(ctx, "account-a", "tenant-a")
	if err != nil || len(empty) != 0 {
		t.Fatalf("ListEmployees() empty = %+v, %v", empty, err)
	}

	created, err := service.CreateEmployee(ctx, "account-a", CreateEmployeeRequest{Name: " 新 AI 客服员工 "})
	if err != nil {
		t.Fatalf("CreateEmployee() error = %v", err)
	}
	if created.EmployeeID == "" || created.Name != "新 AI 客服员工" || created.Status != "paused" || created.Mode != "basic" || created.Prompt != "" || created.History != 10 || !created.AutoReply || len(created.AccountBindings) != 0 {
		t.Fatalf("CreateEmployee() = %+v", created)
	}

	if _, err := service.CreateEmployee(ctx, "account-a", CreateEmployeeRequest{Name: "新 ai 客服员工"}); !isStatus(err, 409) {
		t.Fatalf("duplicate name error = %v", err)
	}
	other, err := service.CreateEmployee(ctx, "account-b", CreateEmployeeRequest{Name: "新 AI 客服员工"})
	if err != nil || other.EmployeeID == created.EmployeeID {
		t.Fatalf("other account create = %+v, %v", other, err)
	}
	hidden, err := service.ListEmployees(ctx, "account-b", "tenant-a")
	if err != nil || len(hidden) != 1 || hidden[0].EmployeeID != other.EmployeeID {
		t.Fatalf("other account list = %+v, %v", hidden, err)
	}

	accounts, err := service.ListEmployeeAccounts(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("ListEmployeeAccounts() error = %v", err)
	}
	if !hasAccount(accounts, "whatsapp", "app_sales") || !hasAccount(accounts, "web", channel.ChannelKey) || hasAccount(accounts, "telegram", "app_other") {
		t.Fatalf("ListEmployeeAccounts() = %+v", accounts)
	}

	updated, err := service.UpdateEmployee(ctx, "account-a", "tenant-a", created.EmployeeID, UpdateEmployeeRequest{
		Name: "售后专员", Persona: "耐心", Status: "running", Mode: "professional", Prompt: "只回答售后问题",
		History: 15, AutoReply: false, SelectedDocIDs: []string{"d3", "d3"}, Tools: []string{"查询订单"},
		AccountBindings: []AccountBinding{{Platform: "WhatsApp", AccountKey: "app_sales"}, {Platform: "web", AccountKey: channel.ChannelKey}},
	})
	if err != nil {
		t.Fatalf("UpdateEmployee() error = %v", err)
	}
	if updated.Name != "售后专员" || updated.Status != "running" || updated.AutoReply || len(updated.SelectedDocIDs) != 1 || len(updated.AccountBindings) != 2 {
		t.Fatalf("UpdateEmployee() = %+v", updated)
	}
	stored, err := service.ListEmployees(ctx, "account-a", "tenant-a")
	if err != nil || len(stored) != 1 || stored[0].Prompt != "只回答售后问题" || len(stored[0].SelectedDocIDs) != 1 || stored[0].SelectedDocIDs[0] != "d3" || stored[0].AutoReply || len(stored[0].AccountBindings) != 2 {
		t.Fatalf("stored employee = %+v, %v", stored, err)
	}

	if _, err := service.UpdateEmployee(ctx, "account-b", "tenant-a", created.EmployeeID, UpdateEmployeeRequest{
		Name: "偷看", Status: "paused", Mode: "basic", History: 10,
	}); !isStatus(err, 404) {
		t.Fatalf("cross account update error = %v", err)
	}
	second, err := service.CreateEmployee(ctx, "account-a", CreateEmployeeRequest{Name: "售前助手"})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if _, err := service.UpdateEmployee(ctx, "account-a", "tenant-a", second.EmployeeID, UpdateEmployeeRequest{
		Name: "售前助手", Status: "paused", Mode: "basic", History: 10,
		AccountBindings: []AccountBinding{{Platform: "whatsapp", AccountKey: "app_sales"}},
	}); !isStatus(err, 409) {
		t.Fatalf("binding conflict error = %v", err)
	}
	if _, err := service.UpdateEmployee(ctx, "account-a", "tenant-a", second.EmployeeID, UpdateEmployeeRequest{
		Name: "售前助手", Status: "paused", Mode: "basic", History: 10,
		AccountBindings: []AccountBinding{{Platform: "telegram", AccountKey: "app_other"}},
	}); !isStatus(err, 400) {
		t.Fatalf("unknown account error = %v", err)
	}
	if _, err := service.UpdateEmployee(ctx, "account-a", "tenant-a", second.EmployeeID, UpdateEmployeeRequest{
		Name: "售前助手", Status: "paused", Mode: "basic", History: 0,
	}); !isStatus(err, 400) {
		t.Fatalf("history error = %v", err)
	}

	if err := service.DeleteEmployee(ctx, "account-b", created.EmployeeID); !isStatus(err, 404) {
		t.Fatalf("cross account delete error = %v", err)
	}
	if err := service.DeleteEmployee(ctx, "account-a", created.EmployeeID); err != nil {
		t.Fatalf("DeleteEmployee() error = %v", err)
	}
	remaining, err := service.ListEmployees(ctx, "account-a", "tenant-a")
	if err != nil || len(remaining) != 1 || remaining[0].EmployeeID != second.EmployeeID {
		t.Fatalf("list after delete = %+v, %v", remaining, err)
	}
	recreated, err := service.CreateEmployee(ctx, "account-a", CreateEmployeeRequest{Name: "售后专员"})
	if err != nil || recreated.EmployeeID == "" {
		t.Fatalf("recreate deleted name = %+v, %v", recreated, err)
	}
	rebound, err := service.UpdateEmployee(ctx, "account-a", "tenant-a", recreated.EmployeeID, UpdateEmployeeRequest{
		Name: "售后专员", Status: "paused", Mode: "basic", History: 10,
		AccountBindings: []AccountBinding{{Platform: "whatsapp", AccountKey: "app_sales"}},
	})
	if err != nil || len(rebound.AccountBindings) != 1 || rebound.AccountBindings[0].Label != "售前业务" {
		t.Fatalf("rebind after delete = %+v, %v", rebound, err)
	}
}

func hasAccount(items []AccountBinding, platform, accountKey string) bool {
	for _, item := range items {
		if item.Platform == platform && item.AccountKey == accountKey {
			return true
		}
	}
	return false
}

func isStatus(err error, status int) bool {
	var target *Error
	return errors.As(err, &target) && target.Status == status
}
