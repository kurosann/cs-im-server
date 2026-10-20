package service

import (
	"context"
	"testing"
	"time"
)

func TestEmployeeExecutionsPersistAndIsolateByAccount(t *testing.T) {
	ctx := context.Background()
	service := setupEmployees(t)
	first, err := service.CreateEmployee(ctx, "account-a", CreateEmployeeRequest{Name: "售后专员"})
	if err != nil {
		t.Fatalf("create employee: %v", err)
	}
	other, err := service.CreateEmployee(ctx, "account-b", CreateEmployeeRequest{Name: "售后专员"})
	if err != nil {
		t.Fatalf("create other employee: %v", err)
	}

	historyError := "history_unavailable"
	completed := time.Date(2026, 10, 20, 12, 34, 0, 0, time.UTC)
	saved, err := service.RecordEmployeeExecution(ctx, "account-a", first.EmployeeID, RecordEmployeeExecutionInput{
		Platform:           "whatsapp",
		AccountKey:         "wa-1",
		AccountDisplayName: "售前 WhatsApp",
		ProfileID:          "profile-1",
		PeerID:             "peer-1",
		ConversationID:     "conv-1",
		PeerDisplayName:    "Maya",
		MessageIDs:         []string{"m1", " m1 ", "m2"},
		MergedText:         "退款多久到账？",
		HistoryCount:       3,
		HistoryAvailable:   false,
		HistoryTruncated:   true,
		HistoryDiagnostics: &HistoryDiagnostics{
			Status: "unavailable", RequestedCount: 10, ReadCount: 0, Pages: 1, ScannedCount: 4, Error: &historyError,
		},
		Status:           "replied",
		Action:           "reply",
		Reason:           "命中退款政策",
		ReplyText:        "预计 1-2 个工作日到账。",
		PromptTokens:     120,
		CompletionTokens: 40,
		TotalTokens:      160,
		Result:           "sent",
		CompletedAt:      &completed,
	})
	if err != nil {
		t.Fatalf("record execution: %v", err)
	}
	if saved.EmployeeID != first.EmployeeID || saved.EmployeeName != "售后专员" || saved.Status != "replied" {
		t.Fatalf("saved view = %+v", saved)
	}
	if len(saved.MessageIDs) != 2 || saved.MessageIDs[0] != "m1" || saved.MessageIDs[1] != "m2" {
		t.Fatalf("message ids = %#v", saved.MessageIDs)
	}
	if saved.AccountDisplayName == nil || *saved.AccountDisplayName != "售前 WhatsApp" || saved.PeerDisplayName == nil || *saved.PeerDisplayName != "Maya" {
		t.Fatalf("names = %+v", saved)
	}
	if saved.HistoryDiagnostics == nil || saved.HistoryDiagnostics.Status != "unavailable" || saved.HistoryDiagnostics.Error == nil {
		t.Fatalf("diagnostics = %+v", saved.HistoryDiagnostics)
	}
	if saved.CompletedAt == nil || *saved.CompletedAt != completed.UnixMilli() || saved.TotalTokens != 160 {
		t.Fatalf("completion = %+v tokens %d", saved.CompletedAt, saved.TotalTokens)
	}

	listed, err := service.ListEmployeeExecutions(ctx, "account-a", first.EmployeeID, 20)
	if err != nil || len(listed) != 1 || listed[0].ID != saved.ID || listed[0].MergedText != "退款多久到账？" {
		t.Fatalf("listed = %+v err %v", listed, err)
	}
	if _, err := service.ListEmployeeExecutions(ctx, "account-b", first.EmployeeID, 20); err == nil || !isStatus(err, 404) {
		t.Fatalf("other account err = %v", err)
	}
	empty, err := service.ListEmployeeExecutions(ctx, "account-b", other.EmployeeID, 20)
	if err != nil || len(empty) != 0 {
		t.Fatalf("other employee logs = %+v err %v", empty, err)
	}
	if _, err := service.RecordEmployeeExecution(ctx, "account-a", first.EmployeeID, RecordEmployeeExecutionInput{
		Platform: "whatsapp", Status: "sent",
	}); err == nil || !isStatus(err, 400) {
		t.Fatalf("invalid status err = %v", err)
	}
}
