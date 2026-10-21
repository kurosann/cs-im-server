package service

import (
	"context"
	"strings"
	"time"
)

type TestEmployeeRequest struct {
	Prompt         string            `json:"prompt"`
	Persona        string            `json:"persona"`
	History        int               `json:"history"`
	AutoReply      bool              `json:"autoReply"`
	SelectedDocIDs []string          `json:"selectedDocIds"`
	Message        string            `json:"message"`
	HistoryTurns   []DecisionMessage `json:"historyTurns"`
}

type TestEmployeeResult struct {
	Action     string   `json:"action"`
	ReplyText  string   `json:"replyText"`
	ReasonCode string   `json:"reasonCode"`
	Reason     string   `json:"reason"`
	Knowledge  []string `json:"knowledge"`
	Risk       string   `json:"risk"`
	Error      string   `json:"error,omitempty"`
}

func (s *RelayService) TestEmployee(ctx context.Context, accountKey, tenantID, employeeID string, req TestEmployeeRequest) (TestEmployeeResult, error) {
	accountKey = strings.TrimSpace(accountKey)
	employee, err := s.findEmployee(ctx, accountKey, employeeID)
	if err != nil {
		return TestEmployeeResult{}, err
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return TestEmployeeResult{}, badRequest("客户消息不能为空")
	}
	limit := req.History
	if limit < employeeHistoryMin || limit > employeeHistoryMax {
		limit = employee.HistoryMessageCount
	}
	if limit < employeeHistoryMin || limit > employeeHistoryMax {
		limit = employeeHistoryDefault
	}
	turns := normalizeDecisionMessages(req.HistoryTurns)
	if len(turns) > limit {
		turns = turns[len(turns)-limit:]
	}
	docIDs := req.SelectedDocIDs
	if docIDs == nil {
		docIDs = employee.SelectedDocIDs
	}
	tenantID = firstNonEmpty(strings.TrimSpace(tenantID), employee.TenantID)
	facts, err := s.KnowledgeFacts(ctx, tenantID, docIDs, "", false)
	if err != nil {
		return TestEmployeeResult{}, err
	}
	input := DecisionInput{
		Platform:    "test",
		Persona:     req.Persona,
		Prompt:      req.Prompt,
		Knowledge:   facts,
		AutoReply:   req.AutoReply,
		History:     turns,
		NewMessages: []DecisionMessage{{Role: "customer", Text: message}},
	}
	assembled := assembleDecision(input)
	result := decideAIReply(ctx, s.completer, input)
	status, resultText := testExecutionStatus(result)
	if result.Action == "escalate" && !result.Failed {
		if err := reportControlCenter(ctx, ControlCenterReport{
			EmployeeID: employee.EmployeeKey, Platform: "test", ReasonCode: result.ReasonCode, Reason: result.Reason, ReplyText: result.ReplyText,
		}); err != nil {
			status = "not_reported"
			resultText = "control_center_reporting_disabled"
		}
	}
	diagnostics := &HistoryDiagnostics{Status: "complete", RequestedCount: limit, ReadCount: assembled.Meta.HistoryCount, Pages: 1, ScannedCount: len(req.HistoryTurns)}
	if assembled.Meta.HistoryCount == 0 {
		diagnostics.Status = "empty"
	} else if assembled.Meta.HistoryTruncated || len(req.HistoryTurns) > len(turns) {
		diagnostics.Status = "partial"
	}
	completed := time.Now()
	if _, err := s.RecordEmployeeExecution(ctx, accountKey, employee.EmployeeKey, RecordEmployeeExecutionInput{
		Platform:           "test",
		AccountKey:         "test",
		ProfileID:          "test",
		PeerID:             "test",
		ConversationID:     "test",
		MessageIDs:         []string{},
		MergedText:         message,
		HistoryCount:       assembled.Meta.HistoryCount,
		HistoryAvailable:   assembled.Meta.HistoryAvailable,
		HistoryTruncated:   assembled.Meta.HistoryTruncated,
		HistoryDiagnostics: diagnostics,
		Status:             status,
		Action:             result.Action,
		ReasonCode:         result.ReasonCode,
		Reason:             result.Reason,
		ReplyText:          result.ReplyText,
		PromptTokens:       result.PromptTokens,
		CompletionTokens:   result.CompletionTokens,
		TotalTokens:        result.PromptTokens + result.CompletionTokens,
		Result:             resultText,
		Error:              result.FailureMessage,
		CompletedAt:        &completed,
	}); err != nil {
		return TestEmployeeResult{}, err
	}
	knowledge := assembled.Meta.KnowledgeTitles
	if knowledge == nil {
		knowledge = []string{}
	}
	return TestEmployeeResult{
		Action:     result.Action,
		ReplyText:  result.ReplyText,
		ReasonCode: result.ReasonCode,
		Reason:     result.Reason,
		Knowledge:  knowledge,
		Risk:       riskLabel(result),
		Error:      result.FailureMessage,
	}, nil
}

func testExecutionStatus(result DecisionResult) (string, string) {
	if result.Failed {
		return "failed", ""
	}
	switch result.Action {
	case "reply":
		return "replied", ""
	case "ignore":
		return "ignored", ""
	case "escalate":
		return "not_reported", "control_center_reporting_disabled"
	default:
		return "failed", ""
	}
}

func riskLabel(result DecisionResult) string {
	if result.Failed || result.Action == "escalate" {
		return "高"
	}
	return "低"
}
