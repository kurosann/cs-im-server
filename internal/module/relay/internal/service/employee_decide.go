package service

import (
	"context"
	"strings"
	"time"
)

type ControlCenterReport struct {
	ExecutionID    string
	EmployeeID     string
	Platform       string
	ConversationID string
	CustomerID     string
	ReasonCode     string
	Reason         string
	ReplyText      string
}

func reportControlCenter(context.Context, ControlCenterReport) error {
	return errControlCenterDisabled
}

var errControlCenterDisabled = errString("control center reporting disabled")

type errString string

func (e errString) Error() string { return string(e) }

func (s *RelayService) decideAndRecord(ctx context.Context, gateway InboundGateway, employee *AIEmployee, app *businessAppRow, ref ConversationRef, platform string, messages []InboundMessage, filterChannel bool) error {
	newMessages := make([]DecisionMessage, 0, len(messages))
	ids := make([]string, 0, len(messages))
	texts := make([]string, 0, len(messages))
	exclude := make(map[string]struct{}, len(messages))
	for _, message := range messages {
		text := messageMarker(message.Type, message.Text)
		if text == "" {
			continue
		}
		exclude[message.ID] = struct{}{}
		ids = append(ids, message.ID)
		texts = append(texts, text)
		newMessages = append(newMessages, DecisionMessage{Role: "customer", Text: text, MessageID: message.ID})
	}
	if len(newMessages) == 0 {
		return nil
	}
	limit := employee.HistoryMessageCount
	if limit < employeeHistoryMin || limit > employeeHistoryMax {
		limit = employeeHistoryDefault
	}
	history, diagnostics := s.readHistory(ctx, gateway, messages[0].ConversationID, limit, exclude)
	facts, err := s.KnowledgeFacts(ctx, app.TenantID, employee.SelectedDocIDs, platform, filterChannel)
	if err != nil {
		return err
	}
	input := DecisionInput{
		Platform:    platform,
		Persona:     employee.Persona,
		Prompt:      employee.Prompt,
		Knowledge:   facts,
		AutoReply:   employee.AutoReplyEnabled,
		History:     history,
		NewMessages: newMessages,
	}
	assembled := assembleDecision(input)
	if assembled.Meta.HistoryTruncated && diagnostics.Status == "complete" {
		diagnostics.Status = "partial"
	}
	diagnostics.ReadCount = assembled.Meta.HistoryCount
	result := decideAIReply(ctx, s.completer, input)
	reply, shouldSend := outboundReply(result)
	var sendErr error
	if shouldSend && gateway != nil {
		sendErr = gateway.SendAIReply(ctx, messages[0].ConversationID, employee.EmployeeKey, reply)
	}
	status, resultText := executionOutcome(result, shouldSend, sendErr)
	if result.Action == "escalate" && !result.Failed && sendErr == nil {
		if err := reportControlCenter(ctx, ControlCenterReport{
			EmployeeID: employee.EmployeeKey, Platform: platform, ConversationID: messages[0].ConversationID,
			CustomerID: ref.CustomerID, ReasonCode: result.ReasonCode, Reason: result.Reason, ReplyText: result.ReplyText,
		}); err != nil {
			status = "not_reported"
			resultText = "control_center_reporting_disabled"
		} else {
			status = "reported"
			resultText = ""
		}
	}
	completed := time.Now()
	_, err = s.RecordEmployeeExecution(ctx, employee.AccountKey, employee.EmployeeKey, RecordEmployeeExecutionInput{
		Platform:           platform,
		AccountKey:         app.AppKey,
		AccountDisplayName: app.DisplayName,
		ProfileID:          app.AppKey,
		PeerID:             ref.CustomerID,
		ConversationID:     messages[0].ConversationID,
		PeerDisplayName:    ref.Subject,
		MessageIDs:         ids,
		MergedText:         strings.Join(texts, "\n"),
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
		Error:              firstNonEmpty(result.FailureMessage, errorText(sendErr)),
		CompletedAt:        &completed,
	})
	return err
}

func (s *RelayService) readHistory(ctx context.Context, gateway InboundGateway, conversationID string, limit int, exclude map[string]struct{}) ([]DecisionMessage, *HistoryDiagnostics) {
	diagnostics := &HistoryDiagnostics{Status: "complete", RequestedCount: limit, Pages: 1}
	if gateway == nil {
		diagnostics.Status = "unavailable"
		return nil, diagnostics
	}
	raw, err := gateway.RecentMessages(ctx, conversationID, limit+len(exclude)+8)
	if err != nil {
		message := err.Error()
		diagnostics.Status = "unavailable"
		diagnostics.Error = &message
		return nil, diagnostics
	}
	return transcriptHistory(raw, exclude, limit, diagnostics), diagnostics
}

func transcriptHistory(raw []TranscriptMessage, exclude map[string]struct{}, limit int, diagnostics *HistoryDiagnostics) []DecisionMessage {
	diagnostics.ScannedCount = len(raw)
	history := make([]DecisionMessage, 0, len(raw))
	for _, message := range raw {
		if _, skip := exclude[message.ID]; skip {
			continue
		}
		role := ""
		switch message.Role {
		case "customer":
			role = "customer"
		case "agent", "ai":
			role = "business"
		default:
			continue
		}
		text := messageMarker(message.Type, message.Text)
		if text == "" {
			continue
		}
		history = append(history, DecisionMessage{Role: role, Text: text, MessageID: message.ID})
	}
	if len(history) > limit {
		history = history[len(history)-limit:]
		diagnostics.Status = "partial"
	}
	diagnostics.ReadCount = len(history)
	if len(history) == 0 && diagnostics.Status == "complete" {
		diagnostics.Status = "empty"
	}
	return history
}

func executionOutcome(result DecisionResult, shouldSend bool, sendErr error) (string, string) {
	if result.Failed || sendErr != nil {
		return "failed", ""
	}
	switch result.Action {
	case "reply":
		if shouldSend {
			return "replied", ""
		}
		return "failed", ""
	case "ignore":
		return "ignored", ""
	case "escalate":
		return "not_reported", "control_center_reporting_disabled"
	default:
		return "failed", ""
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
