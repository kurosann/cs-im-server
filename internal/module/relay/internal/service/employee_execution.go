package service

import (
	"context"
	"strings"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"
)

const employeeExecutionListMax = 200

var employeeExecutionStatuses = map[string]struct{}{
	"pending": {}, "analyzing": {}, "replying": {}, "reporting": {},
	"ignored": {}, "replied": {}, "reported": {}, "not_reported": {},
	"failed": {}, "cancelled": {},
}

type EmployeeExecutionView struct {
	ID                 string              `json:"id"`
	EmployeeID         string              `json:"employeeId"`
	EmployeeName       string              `json:"employeeName"`
	Platform           string              `json:"platform"`
	AccountKey         string              `json:"accountKey"`
	AccountDisplayName *string             `json:"accountDisplayName"`
	ProfileID          string              `json:"profileId"`
	PeerID             string              `json:"peerId"`
	ConversationID     string              `json:"conversationId"`
	PeerDisplayName    *string             `json:"peerDisplayName"`
	MessageIDs         []string            `json:"messageIds"`
	MergedText         string              `json:"mergedText"`
	HistoryCount       int                 `json:"historyCount"`
	HistoryAvailable   bool                `json:"historyAvailable"`
	HistoryTruncated   bool                `json:"historyTruncated"`
	HistoryDiagnostics *HistoryDiagnostics `json:"historyDiagnostics"`
	Status             string              `json:"status"`
	Action             *string             `json:"action"`
	ReasonCode         *string             `json:"reasonCode"`
	Reason             *string             `json:"reason"`
	ReplyText          *string             `json:"replyText"`
	PromptTokens       int                 `json:"promptTokens"`
	CompletionTokens   int                 `json:"completionTokens"`
	TotalTokens        int                 `json:"totalTokens"`
	Result             *string             `json:"result"`
	Error              *string             `json:"error"`
	CreatedAt          int64               `json:"createdAt"`
	UpdatedAt          int64               `json:"updatedAt"`
	CompletedAt        *int64              `json:"completedAt"`
}

type RecordEmployeeExecutionInput struct {
	Platform           string
	AccountKey         string
	AccountDisplayName string
	ProfileID          string
	PeerID             string
	ConversationID     string
	PeerDisplayName    string
	MessageIDs         []string
	MergedText         string
	HistoryCount       int
	HistoryAvailable   bool
	HistoryTruncated   bool
	HistoryDiagnostics *HistoryDiagnostics
	Status             string
	Action             string
	ReasonCode         string
	Reason             string
	ReplyText          string
	PromptTokens       int
	CompletionTokens   int
	TotalTokens        int
	Result             string
	Error              string
	CompletedAt        *time.Time
}

func (s *RelayService) ListEmployeeExecutions(ctx context.Context, accountKey, employeeID string, limit int) ([]EmployeeExecutionView, error) {
	accountKey = strings.TrimSpace(accountKey)
	employeeID = strings.TrimSpace(employeeID)
	if accountKey == "" {
		return nil, unauthorized("请先登录")
	}
	if _, err := s.findEmployee(ctx, accountKey, employeeID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > employeeExecutionListMax {
		limit = employeeExecutionListMax
	}
	items, err := s.executions.List(ctx,
		dataaccess.Eq("account_key", accountKey),
		dataaccess.Eq("employee_key", employeeID),
		dataaccess.Order("create_at", dataaccess.OrderTypeDesc),
		dataaccess.Order("id", dataaccess.OrderTypeDesc),
		dataaccess.Limit(limit),
	)
	if err != nil {
		return nil, err
	}
	views := make([]EmployeeExecutionView, 0, len(items))
	for _, item := range items {
		views = append(views, employeeExecutionView(item))
	}
	return views, nil
}

func (s *RelayService) RecordEmployeeExecution(ctx context.Context, accountKey, employeeID string, input RecordEmployeeExecutionInput) (EmployeeExecutionView, error) {
	accountKey = strings.TrimSpace(accountKey)
	employeeID = strings.TrimSpace(employeeID)
	if accountKey == "" {
		return EmployeeExecutionView{}, unauthorized("请先登录")
	}
	employee, err := s.findEmployee(ctx, accountKey, employeeID)
	if err != nil {
		return EmployeeExecutionView{}, err
	}
	status := strings.TrimSpace(input.Status)
	if _, ok := employeeExecutionStatuses[status]; !ok {
		return EmployeeExecutionView{}, badRequest("不支持的执行状态")
	}
	platform := strings.TrimSpace(input.Platform)
	if platform == "" {
		return EmployeeExecutionView{}, badRequest("平台不能为空")
	}
	if input.HistoryCount < 0 || input.PromptTokens < 0 || input.CompletionTokens < 0 || input.TotalTokens < 0 {
		return EmployeeExecutionView{}, badRequest("用量不能为负数")
	}
	diagnostics, err := normalizeHistoryDiagnostics(input.HistoryDiagnostics)
	if err != nil {
		return EmployeeExecutionView{}, err
	}
	item := &AIEmployeeExecution{
		ExecutionKey:       "exe_" + idutil.GenShortId(),
		AccountKey:         accountKey,
		EmployeeKey:        employee.EmployeeKey,
		EmployeeName:       employee.Name,
		Platform:           platform,
		SessionAccountKey:  strings.TrimSpace(input.AccountKey),
		AccountDisplayName: optionalText(input.AccountDisplayName),
		ProfileID:          strings.TrimSpace(input.ProfileID),
		PeerID:             strings.TrimSpace(input.PeerID),
		ConversationID:     strings.TrimSpace(input.ConversationID),
		PeerDisplayName:    optionalText(input.PeerDisplayName),
		MessageIDs:         normalizeStringList(input.MessageIDs),
		MergedText:         strings.TrimSpace(input.MergedText),
		HistoryCount:       input.HistoryCount,
		HistoryAvailable:   input.HistoryAvailable,
		HistoryTruncated:   input.HistoryTruncated,
		HistoryDiagnostics: diagnostics,
		Status:             status,
		Action:             optionalText(input.Action),
		ReasonCode:         optionalText(input.ReasonCode),
		Reason:             optionalText(input.Reason),
		ReplyText:          optionalText(input.ReplyText),
		PromptTokens:       input.PromptTokens,
		CompletionTokens:   input.CompletionTokens,
		TotalTokens:        input.TotalTokens,
		Result:             optionalText(input.Result),
		ErrorMessage:       optionalText(input.Error),
		CompletedAt:        input.CompletedAt,
	}
	if err := s.executions.Create(ctx, item); err != nil {
		return EmployeeExecutionView{}, err
	}
	return employeeExecutionView(item), nil
}

func normalizeHistoryDiagnostics(value *HistoryDiagnostics) (*HistoryDiagnostics, error) {
	if value == nil {
		return nil, nil
	}
	switch value.Status {
	case "complete", "empty", "partial", "unavailable":
	default:
		return nil, badRequest("不支持的历史读取状态")
	}
	if value.RequestedCount < 0 || value.ReadCount < 0 || value.Pages < 0 || value.ScannedCount < 0 {
		return nil, badRequest("历史读取数据不正确")
	}
	out := *value
	if out.Error != nil {
		trimmed := strings.TrimSpace(*out.Error)
		if trimmed == "" {
			out.Error = nil
		} else {
			out.Error = &trimmed
		}
	}
	return &out, nil
}

func optionalText(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func employeeExecutionView(item *AIEmployeeExecution) EmployeeExecutionView {
	messageIDs := item.MessageIDs
	if messageIDs == nil {
		messageIDs = []string{}
	}
	view := EmployeeExecutionView{
		ID:                 item.ExecutionKey,
		EmployeeID:         item.EmployeeKey,
		EmployeeName:       item.EmployeeName,
		Platform:           item.Platform,
		AccountKey:         item.SessionAccountKey,
		AccountDisplayName: item.AccountDisplayName,
		ProfileID:          item.ProfileID,
		PeerID:             item.PeerID,
		ConversationID:     item.ConversationID,
		PeerDisplayName:    item.PeerDisplayName,
		MessageIDs:         messageIDs,
		MergedText:         item.MergedText,
		HistoryCount:       item.HistoryCount,
		HistoryAvailable:   item.HistoryAvailable,
		HistoryTruncated:   item.HistoryTruncated,
		HistoryDiagnostics: item.HistoryDiagnostics,
		Status:             item.Status,
		Action:             item.Action,
		ReasonCode:         item.ReasonCode,
		Reason:             item.Reason,
		ReplyText:          item.ReplyText,
		PromptTokens:       item.PromptTokens,
		CompletionTokens:   item.CompletionTokens,
		TotalTokens:        item.TotalTokens,
		Result:             item.Result,
		Error:              item.ErrorMessage,
		CreatedAt:          unixMilli(item.CreateAt),
		UpdatedAt:          unixMilli(item.UpdateAt),
	}
	if item.CompletedAt != nil {
		completed := item.CompletedAt.UnixMilli()
		view.CompletedAt = &completed
	}
	return view
}

func unixMilli(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}
	return value.UnixMilli()
}
