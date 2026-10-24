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
	if err := db.AutoMigrate(&Channel{}, &AIEmployee{}, &AIEmployeeBinding{}, &AIEmployeeExecution{}, &businessAppRow{}, &KnowledgeGroup{}, &KnowledgeBase{}, &KnowledgeDoc{}, &iamUserRow{}); err != nil {
		t.Fatalf("migrate sqlite: %v", err)
	}
	return NewRelayService(RuntimeDeps{Realtime: realtimePublisherStub{}})
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
	if _, err := service.CreateChannel(ctx, relaydto.CreateChannelRequest{Name: "官网转发", Type: "web"}); err != nil {
		t.Fatalf("create channel: %v", err)
	}

	empty, err := service.ListEmployees(ctx, "account-a", "tenant-a")
	if err != nil || len(empty) != 0 {
		t.Fatalf("ListEmployees() empty = %+v, %v", empty, err)
	}

	created, err := service.CreateEmployee(ctx, "account-a", "tenant-a", CreateEmployeeRequest{Name: " 新 AI 客服员工 "})
	if err != nil {
		t.Fatalf("CreateEmployee() error = %v", err)
	}
	if created.EmployeeID == "" || created.Name != "新 AI 客服员工" || created.Status != "paused" || created.Mode != "basic" || created.Prompt != "" || created.History != 10 || !created.AutoReply || len(created.Channels) != 0 {
		t.Fatalf("CreateEmployee() = %+v", created)
	}

	if _, err := service.CreateEmployee(ctx, "account-a", "tenant-a", CreateEmployeeRequest{Name: "新 ai 客服员工"}); !isStatus(err, 409) {
		t.Fatalf("duplicate name error = %v", err)
	}
	other, err := service.CreateEmployee(ctx, "account-b", "tenant-b", CreateEmployeeRequest{Name: "新 AI 客服员工"})
	if err != nil || other.EmployeeID == created.EmployeeID {
		t.Fatalf("other account create = %+v, %v", other, err)
	}
	hidden, err := service.ListEmployees(ctx, "account-b", "tenant-a")
	if err != nil || len(hidden) != 1 || hidden[0].EmployeeID != other.EmployeeID {
		t.Fatalf("other account list = %+v, %v", hidden, err)
	}

	if err := service.apps.Create(ctx, &businessAppRow{
		AppKey: "app_sales_2", TenantID: "tenant-a", DisplayName: "售后业务", Kind: "whatsapp",
	}); err != nil {
		t.Fatalf("create second app: %v", err)
	}
	channels, err := service.ListEmployeeChannels(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("ListEmployeeChannels() error = %v", err)
	}
	if len(channels) != 2 || !hasChannel(channels, "whatsapp") || !hasChannel(channels, "web") || hasChannel(channels, "telegram") {
		t.Fatalf("ListEmployeeChannels() = %+v", channels)
	}
	if label := channelLabelOf(channels, "whatsapp"); label != "WhatsApp" {
		t.Fatalf("whatsapp label = %q", label)
	}

	updated, err := service.UpdateEmployee(ctx, "account-a", "tenant-a", created.EmployeeID, UpdateEmployeeRequest{
		Name: "售后专员", Persona: "耐心", Status: "running", Mode: "professional", Prompt: "只回答售后问题",
		History: 15, AutoReply: false, SelectedDocIDs: []string{"d3", "d3"}, Tools: []string{"查询订单"},
		Channels: []ChannelBinding{{Platform: "WhatsApp"}, {Platform: "web"}},
	})
	if err != nil {
		t.Fatalf("UpdateEmployee() error = %v", err)
	}
	if updated.Name != "售后专员" || updated.Status != "running" || updated.AutoReply || len(updated.SelectedDocIDs) != 1 || len(updated.Channels) != 2 {
		t.Fatalf("UpdateEmployee() = %+v", updated)
	}
	stored, err := service.ListEmployees(ctx, "account-a", "tenant-a")
	if err != nil || len(stored) != 1 || stored[0].Prompt != "只回答售后问题" || len(stored[0].SelectedDocIDs) != 1 || stored[0].SelectedDocIDs[0] != "d3" || stored[0].AutoReply || len(stored[0].Channels) != 2 {
		t.Fatalf("stored employee = %+v, %v", stored, err)
	}

	if _, err := service.UpdateEmployee(ctx, "account-b", "tenant-a", created.EmployeeID, UpdateEmployeeRequest{
		Name: "偷看", Status: "paused", Mode: "basic", History: 10,
	}); !isStatus(err, 404) {
		t.Fatalf("cross account update error = %v", err)
	}
	second, err := service.CreateEmployee(ctx, "account-a", "tenant-a", CreateEmployeeRequest{Name: "售前助手"})
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	if _, err := service.UpdateEmployee(ctx, "account-a", "tenant-a", second.EmployeeID, UpdateEmployeeRequest{
		Name: "售前助手", Status: "paused", Mode: "basic", History: 10,
		Channels: []ChannelBinding{{Platform: "whatsapp"}},
	}); !isStatus(err, 409) {
		t.Fatalf("binding conflict error = %v", err)
	}
	if _, err := service.UpdateEmployee(ctx, "account-a", "tenant-a", second.EmployeeID, UpdateEmployeeRequest{
		Name: "售前助手", Status: "paused", Mode: "basic", History: 10,
		Channels: []ChannelBinding{{Platform: "telegram"}},
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
	recreated, err := service.CreateEmployee(ctx, "account-a", "tenant-a", CreateEmployeeRequest{Name: "售后专员"})
	if err != nil || recreated.EmployeeID == "" {
		t.Fatalf("recreate deleted name = %+v, %v", recreated, err)
	}
	rebound, err := service.UpdateEmployee(ctx, "account-a", "tenant-a", recreated.EmployeeID, UpdateEmployeeRequest{
		Name: "售后专员", Status: "paused", Mode: "basic", History: 10,
		Channels: []ChannelBinding{{Platform: "whatsapp"}},
	})
	if err != nil || len(rebound.Channels) != 1 || rebound.Channels[0].Label != "WhatsApp" {
		t.Fatalf("rebind after delete = %+v, %v", rebound, err)
	}
}

func TestEmployeeChannelsCollapseLegacyAccountRows(t *testing.T) {
	ctx := context.Background()
	service := setupEmployees(t)
	if err := service.apps.Create(ctx, &businessAppRow{
		AppKey: "app_sales", TenantID: "tenant-a", DisplayName: "售前业务", Kind: "WhatsApp",
	}); err != nil {
		t.Fatalf("create app: %v", err)
	}
	created, err := service.CreateEmployee(ctx, "account-a", "tenant-a", CreateEmployeeRequest{Name: "售后专员"})
	if err != nil {
		t.Fatalf("CreateEmployee() error = %v", err)
	}
	for _, accountKey := range []string{"app_sales", "app_sales_2"} {
		if err := service.bindings.Create(ctx, &AIEmployeeBinding{
			AccountKey:      "account-a",
			EmployeeKey:     created.EmployeeID,
			Platform:        "whatsapp",
			BoundAccountKey: accountKey,
		}); err != nil {
			t.Fatalf("create legacy binding: %v", err)
		}
	}
	stored, err := service.ListEmployees(ctx, "account-a", "tenant-a")
	if err != nil || len(stored) != 1 || len(stored[0].Channels) != 1 || stored[0].Channels[0].Platform != "whatsapp" || stored[0].Channels[0].Label != "WhatsApp" {
		t.Fatalf("collapsed channels = %+v, %v", stored, err)
	}
}

func hasChannel(items []ChannelBinding, platform string) bool {
	for _, item := range items {
		if item.Platform == platform {
			return true
		}
	}
	return false
}

func channelLabelOf(items []ChannelBinding, platform string) string {
	for _, item := range items {
		if item.Platform == platform {
			return item.Label
		}
	}
	return ""
}

func isStatus(err error, status int) bool {
	var target *Error
	return errors.As(err, &target) && target.Status == status
}
