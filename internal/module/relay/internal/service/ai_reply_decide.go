package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxAIReplyContextChars   = 6000
	maxKnowledgeContextChars = 4000
	maxAIReplyMessages       = 64
)

type DecisionMessage struct {
	Role      string `json:"role"`
	Text      string `json:"text"`
	MessageID string `json:"messageId,omitempty"`
}

type KnowledgeFact struct {
	Title  string `json:"title"`
	Type   string `json:"type"`
	Answer string `json:"answer"`
}

type DecisionInput struct {
	Platform    string
	Persona     string
	Prompt      string
	Knowledge   []KnowledgeFact
	AutoReply   bool
	History     []DecisionMessage
	NewMessages []DecisionMessage
	Model       string
}

type DecisionMeta struct {
	HistoryAvailable bool
	HistoryTruncated bool
	HistoryCount     int
	KnowledgeTitles  []string
	CustomerPrompt   string
}

type DecisionResult struct {
	Action           string
	ReplyText        string
	ReasonCode       string
	Reason           string
	PromptTokens     int
	CompletionTokens int
	Failed           bool
	FailureMessage   string
}

type assembledDecision struct {
	System string
	User   string
	Meta   DecisionMeta
}

func assembleDecision(input DecisionInput) assembledDecision {
	model := strings.TrimSpace(input.Model)
	if model == "" {
		model = aiModelName()
	}
	history, historyTruncated := trimDecisionMessages(normalizeDecisionMessages(input.History), normalizeDecisionMessages(input.NewMessages))
	knowledge, titles := trimKnowledge(input.Knowledge)
	customerPrompt := composeCustomerPrompt(input.Persona, input.Prompt)
	payload := struct {
		Platform         string            `json:"platform"`
		CustomerPrompt   string            `json:"customerPrompt"`
		Knowledge        []KnowledgeFact   `json:"knowledge"`
		AutoReplyEnabled bool              `json:"autoReplyEnabled"`
		HistoryAvailable bool              `json:"historyAvailable"`
		HistoryTruncated bool              `json:"historyTruncated"`
		History          []DecisionMessage `json:"history"`
		NewMessages      []DecisionMessage `json:"newMessages"`
	}{
		Platform:         normalizePlatform(input.Platform),
		CustomerPrompt:   customerPrompt,
		Knowledge:        knowledge,
		AutoReplyEnabled: input.AutoReply,
		HistoryAvailable: len(history) > 0,
		HistoryTruncated: historyTruncated,
		History:          history,
		NewMessages:      normalizeDecisionMessages(input.NewMessages),
	}
	if payload.Platform == "" {
		payload.Platform = "web"
	}
	if payload.History == nil {
		payload.History = []DecisionMessage{}
	}
	if payload.NewMessages == nil {
		payload.NewMessages = []DecisionMessage{}
	}
	encoded, _ := json.Marshal(payload)
	return assembledDecision{
		System: aiReplySystemPrompt(model),
		User:   string(encoded),
		Meta: DecisionMeta{
			HistoryAvailable: payload.HistoryAvailable,
			HistoryTruncated: historyTruncated,
			HistoryCount:     len(history),
			KnowledgeTitles:  titles,
			CustomerPrompt:   customerPrompt,
		},
	}
}

func decideAIReply(ctx context.Context, client CompletionClient, input DecisionInput) DecisionResult {
	if client == nil {
		client = newMiniMaxClient()
	}
	assembled := assembleDecision(input)
	first, err := client.Complete(ctx, assembled.System, assembled.User)
	if err != nil {
		return modelFailure(err, first)
	}
	decision, failure := parseDecision(first.Content)
	decision = coerceDisabledAutoReply(input.AutoReply, decision)
	decision.PromptTokens = first.PromptTokens
	decision.CompletionTokens = first.CompletionTokens
	if failure == "" {
		if code, message := validateDecisionBehavior(input, decision); code != "" {
			failure = message
		}
	}
	if failure == "" {
		return decision
	}
	correctedUser, correctionErr := correctionPrompt(assembled.User, decision, failure, input.AutoReply)
	if correctionErr != nil {
		failed := modelFailure(errors.New(failure), first)
		failed.PromptTokens = first.PromptTokens
		failed.CompletionTokens = first.CompletionTokens
		return failed
	}
	second, err := client.Complete(ctx, assembled.System, correctedUser)
	tokens := first.PromptTokens + second.PromptTokens
	completion := first.CompletionTokens + second.CompletionTokens
	if err != nil {
		failed := modelFailure(err, second)
		failed.PromptTokens = tokens
		failed.CompletionTokens = completion
		return failed
	}
	retried, retryFailure := parseDecision(second.Content)
	retried = coerceDisabledAutoReply(input.AutoReply, retried)
	retried.PromptTokens = tokens
	retried.CompletionTokens = completion
	if retryFailure == "" {
		if _, message := validateDecisionBehavior(input, retried); message != "" {
			retryFailure = message
		}
	}
	if retryFailure != "" {
		failed := modelFailure(errors.New(retryFailure), second)
		failed.PromptTokens = tokens
		failed.CompletionTokens = completion
		return failed
	}
	return retried
}

func composeCustomerPrompt(persona, prompt string) string {
	persona = strings.TrimSpace(persona)
	prompt = strings.TrimSpace(prompt)
	switch {
	case persona == "":
		return prompt
	case prompt == "":
		return "回复性格：" + persona
	default:
		return "回复性格：" + persona + "\n" + prompt
	}
}

func normalizeDecisionMessages(messages []DecisionMessage) []DecisionMessage {
	out := make([]DecisionMessage, 0, len(messages))
	for _, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		if role != "customer" && role != "business" {
			continue
		}
		text := strings.TrimSpace(message.Text)
		if text == "" {
			continue
		}
		out = append(out, DecisionMessage{Role: role, Text: text, MessageID: strings.TrimSpace(message.MessageID)})
	}
	return out
}

func trimDecisionMessages(history, current []DecisionMessage) ([]DecisionMessage, bool) {
	currentRunes := 0
	for _, message := range current {
		currentRunes += utf8.RuneCountInString(message.Text)
	}
	budget := maxAIReplyContextChars - currentRunes
	if budget < 0 {
		budget = 0
	}
	kept := make([]DecisionMessage, 0, len(history))
	used := 0
	truncated := false
	for index := len(history) - 1; index >= 0; index-- {
		message := history[index]
		cost := utf8.RuneCountInString(message.Text)
		if len(kept) >= maxAIReplyMessages-len(current) || used+cost > budget {
			truncated = true
			continue
		}
		used += cost
		kept = append(kept, message)
	}
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	if kept == nil {
		kept = []DecisionMessage{}
	}
	return kept, truncated && len(history) > len(kept)
}

func trimKnowledge(facts []KnowledgeFact) ([]KnowledgeFact, []string) {
	kept := make([]KnowledgeFact, 0, len(facts))
	titles := make([]string, 0, len(facts))
	used := 0
	for _, fact := range facts {
		fact.Title = strings.TrimSpace(fact.Title)
		fact.Type = strings.TrimSpace(fact.Type)
		fact.Answer = strings.TrimSpace(fact.Answer)
		if fact.Title == "" && fact.Answer == "" {
			continue
		}
		cost := utf8.RuneCountInString(fact.Title) + utf8.RuneCountInString(fact.Answer)
		if used+cost > maxKnowledgeContextChars {
			break
		}
		used += cost
		kept = append(kept, fact)
		if fact.Title != "" {
			titles = append(titles, fact.Title)
		}
	}
	if kept == nil {
		kept = []KnowledgeFact{}
	}
	if titles == nil {
		titles = []string{}
	}
	return kept, titles
}

func modelFailure(err error, completion Completion) DecisionResult {
	message := "模型调用失败"
	if err != nil && strings.TrimSpace(err.Error()) != "" {
		message = strings.TrimSpace(err.Error())
	}
	if errors.Is(err, errAINotConfigured) {
		message = errAINotConfigured.Error()
	}
	return DecisionResult{
		Action:           "escalate",
		ReasonCode:       "model_error",
		Reason:           message,
		Failed:           true,
		FailureMessage:   message,
		PromptTokens:     completion.PromptTokens,
		CompletionTokens: completion.CompletionTokens,
	}
}

func correctionPrompt(userPrompt string, previous DecisionResult, failure string, autoReply bool) (string, error) {
	var input map[string]any
	if err := json.Unmarshal([]byte(userPrompt), &input); err != nil {
		return "", err
	}
	instruction := "丢弃上一次回复，根据客户最新消息的意图和语言重新生成，并避免重复近期客服回复。"
	if !autoReply {
		instruction = "自动回复已关闭。无需继续响应时 action=ignore 且 replyText 为空；其余情况 action=escalate 且 replyText 为空。"
	}
	input["serverCorrection"] = map[string]string{
		"previousAction":     previous.Action,
		"previousReasonCode": previous.ReasonCode,
		"previousReply":      previous.ReplyText,
		"failure":            failure,
		"instruction":        instruction,
	}
	encoded, err := json.Marshal(input)
	return string(encoded), err
}

func coerceDisabledAutoReply(autoReply bool, decision DecisionResult) DecisionResult {
	if autoReply || decision.Failed {
		return decision
	}
	decision.ReplyText = ""
	if decision.Action != "reply" {
		return decision
	}
	decision.Action = "escalate"
	switch decision.ReasonCode {
	case "insufficient_context", "complex_or_ambiguous", "human_requested", "sensitive_or_risky":
	default:
		decision.ReasonCode = "insufficient_context"
	}
	decision.Reason = "自动回复已关闭，改为记录升级"
	return decision
}

func parseDecision(content string) (DecisionResult, string) {
	value := strings.TrimSpace(content)
	if value == "" {
		return DecisionResult{}, "模型响应正文为空"
	}
	if strings.Contains(value, "```") {
		return DecisionResult{}, "模型决策不符合固定 JSON 结构"
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(value), &fields) != nil || len(fields) != 4 {
		return DecisionResult{}, "模型决策不符合固定 JSON 结构"
	}
	for _, key := range []string{"action", "replyText", "reasonCode", "reason"} {
		raw, ok := fields[key]
		if !ok {
			return DecisionResult{}, "模型决策不符合固定 JSON 结构"
		}
		encoded := strings.TrimSpace(string(raw))
		if len(encoded) < 2 || encoded[0] != '"' {
			return DecisionResult{}, "模型决策不符合固定 JSON 结构"
		}
	}
	var wire struct {
		Action     string `json:"action"`
		ReplyText  string `json:"replyText"`
		ReasonCode string `json:"reasonCode"`
		Reason     string `json:"reason"`
	}
	if json.Unmarshal([]byte(value), &wire) != nil {
		return DecisionResult{}, "模型决策不符合固定 JSON 结构"
	}
	wire.Action = strings.ToLower(strings.TrimSpace(wire.Action))
	wire.ReplyText = strings.TrimSpace(wire.ReplyText)
	wire.ReasonCode = strings.ToLower(strings.TrimSpace(wire.ReasonCode))
	wire.Reason = strings.TrimSpace(wire.Reason)
	if !validReasonCode(wire.ReasonCode) {
		return DecisionResult{}, "模型返回了未知 reasonCode"
	}
	if wire.Reason == "" {
		return DecisionResult{}, "模型未返回审计原因"
	}
	switch wire.Action {
	case "reply":
		if wire.ReplyText == "" {
			return DecisionResult{}, "模型选择回复但回复正文为空"
		}
	case "ignore":
		if wire.ReplyText != "" {
			return DecisionResult{}, "模型决策不符合固定 JSON 结构"
		}
	case "escalate":
		if wire.ReplyText != "" && wire.ReasonCode != "human_requested" {
			return DecisionResult{}, "模型决策不符合固定 JSON 结构"
		}
	default:
		return DecisionResult{}, "模型返回了未知 action"
	}
	return DecisionResult{
		Action:     wire.Action,
		ReplyText:  wire.ReplyText,
		ReasonCode: wire.ReasonCode,
		Reason:     wire.Reason,
	}, ""
}

func validReasonCode(code string) bool {
	switch code {
	case "prompt_supported", "history_supported", "simple_greeting", "no_reply_needed",
		"insufficient_context", "complex_or_ambiguous", "human_requested", "sensitive_or_risky":
		return true
	default:
		return false
	}
}

func validateDecisionBehavior(input DecisionInput, decision DecisionResult) (string, string) {
	if decision.Action == "" {
		return "", ""
	}
	if !actionReasonCompatible(decision.Action, decision.ReasonCode) {
		return "action_reason_mismatch", "处理动作与原因类别不一致"
	}
	if decision.Action != "reply" {
		return "", ""
	}
	latest := latestCustomerText(input.NewMessages)
	if isLatinLanguageMessage(latest) && isPredominantlyHan(decision.ReplyText) {
		return "reply_language_mismatch", "回复语言与客户最新消息的语言不一致"
	}
	normalizedReply := normalizeReplyText(decision.ReplyText)
	if normalizedReply == "" {
		return "", ""
	}
	for _, message := range input.History {
		if strings.EqualFold(message.Role, "business") && normalizeReplyText(message.Text) == normalizedReply {
			return "reply_repeats_history", "回复与近期客服回复重复"
		}
	}
	return "", ""
}

func actionReasonCompatible(action, reasonCode string) bool {
	switch action {
	case "reply":
		return reasonCode == "prompt_supported" || reasonCode == "history_supported" || reasonCode == "simple_greeting"
	case "ignore":
		return reasonCode == "no_reply_needed"
	case "escalate":
		return reasonCode == "insufficient_context" || reasonCode == "complex_or_ambiguous" || reasonCode == "human_requested" || reasonCode == "sensitive_or_risky"
	default:
		return false
	}
}

func latestCustomerText(messages []DecisionMessage) string {
	for index := len(messages) - 1; index >= 0; index-- {
		if strings.EqualFold(strings.TrimSpace(messages[index].Role), "customer") {
			return strings.TrimSpace(messages[index].Text)
		}
	}
	return ""
}

func isLatinLanguageMessage(text string) bool {
	latin, han := 0, 0
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Latin):
			latin++
		case unicode.In(r, unicode.Han):
			han++
		}
	}
	return latin >= 2 && han == 0
}

func isPredominantlyHan(text string) bool {
	latin, han := 0, 0
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Latin):
			latin++
		case unicode.In(r, unicode.Han):
			han++
		}
	}
	return han >= 2 && han > latin
}

func normalizeReplyText(text string) string {
	var normalized strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			normalized.WriteRune(r)
		}
	}
	return normalized.String()
}

func outboundReply(result DecisionResult) (string, bool) {
	if result.Failed {
		return "", false
	}
	text := strings.TrimSpace(result.ReplyText)
	if text == "" {
		return "", false
	}
	if result.Action == "reply" || result.Action == "escalate" {
		return text, true
	}
	return "", false
}

func messageMarker(messageType, text string) string {
	text = strings.TrimSpace(text)
	var marker string
	switch strings.ToLower(strings.TrimSpace(messageType)) {
	case "image":
		marker = "【图片】"
	case "file":
		marker = "【文件】"
	case "video":
		marker = "【视频】"
	}
	switch {
	case marker == "":
		return text
	case text == "":
		return marker
	default:
		return text + "\n" + marker
	}
}
