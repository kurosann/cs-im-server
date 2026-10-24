package service

import (
	"context"
	"net/url"
	"strings"
	"sync"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"cs-im-server/internal/module/gateway/internal/model"
)

type MessageService struct {
	mu               sync.RWMutex
	historyAccessor  dataaccess.DataAccessor[*model.HistoryRequest]
	accessor         dataaccess.DataAccessor[*model.Message]
	apps             *AppService
	conversations    *ConversationService
	realtime         *RealtimeService
	customerObserver func(model.Message)
}

func validateMessage(req *gatewaydto.CreateMessageRequest) error {
	if !req.Sender.Role.Valid() {
		return badRequest("unsupported sender role %s", req.Sender.Role)
	}
	if req.Status != "" && !req.Status.Valid() {
		return badRequest("unsupported message status %s", req.Status)
	}
	if req.Type == "" {
		req.Type = gatewaydto.MessageTypeText
	}
	if !req.Type.Valid() {
		return badRequest("unsupported message type %s", req.Type)
	}
	if req.Type == gatewaydto.MessageTypeText {
		if req.Text == "" {
			return badRequest("text is required")
		}
		if req.Content != nil {
			return badRequest("content is not allowed for text messages")
		}
		return nil
	}
	if req.Content == nil || req.Content.URL == "" {
		return badRequest("content.url is required")
	}
	parsed, err := url.Parse(req.Content.URL)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return badRequest("content.url must be an absolute http or https URL")
	}
	return nil
}

func NewMessageService(apps *AppService, conversations *ConversationService, realtime *RealtimeService) *MessageService {
	return &MessageService{apps: apps, conversations: conversations, realtime: realtime}
}

func messageSourceDedupKey(appID, externalID string) string {
	if appID == "" || externalID == "" {
		return ""
	}
	return appID + ":" + externalID
}

func (s *MessageService) Create(_ context.Context, req gatewaydto.CreateMessageRequest, publish bool) (model.Message, error) {
	if req.ConversationKey == "" {
		return model.Message{}, badRequest("conversationId is required")
	}
	if req.Sender.Role == "" {
		return model.Message{}, badRequest("sender.role is required")
	}
	if req.Sender.ID == "" {
		return model.Message{}, badRequest("sender.id is required")
	}
	if req.Type == "" {
		req.Type = gatewaydto.MessageTypeText
	}
	if err := validateMessage(&req); err != nil {
		return model.Message{}, err
	}
	conversation, err := s.conversations.Get(context.Background(), req.ConversationKey)
	if err != nil {
		return model.Message{}, err
	}
	if req.CustomerID == "" {
		req.CustomerID = conversation.CustomerKey
	}
	if req.CustomerID != conversation.CustomerKey {
		return model.Message{}, badRequest("customer %s does not belong to conversation", req.CustomerID)
	}
	if conversation.Status != "open" {
		return model.Message{}, badRequest("conversation %s is closed", conversation.ConversationKey)
	}

	sourceKey := messageSourceDedupKey(req.Source.AppID, req.Source.ExternalID)
	s.mu.Lock()
	var existing []model.Message
	if sourceKey != "" {
		found, err := s.listMessages(context.Background(), []dataaccess.QueryCondition{
			dataaccess.Or(
				dataaccess.Eq("source_dedup_key", sourceKey),
				dataaccess.And(
					dataaccess.Eq("source_app_key", req.Source.AppID),
					dataaccess.Eq("source_external_id", req.Source.ExternalID),
				),
			),
			dataaccess.Limit(1),
		}, dataaccess.OrderTypeDesc, 1)
		if err != nil {
			s.mu.Unlock()
			return model.Message{}, err
		}
		existing = found
	}
	if len(existing) != 0 {
		result := existing[0]
		statusChanged := req.Status.Valid() && result.Status != string(req.Status)
		if result.SenderRole != string(req.Sender.Role) || statusChanged {
			result.SenderRole = string(req.Sender.Role)
			result.SenderID = req.Sender.ID
			result.Status = string(req.Status)
			if err := s.accessor.Save(context.Background(), &result); err != nil {
				s.mu.Unlock()
				return model.Message{}, err
			}
		}
		if statusChanged {
			go s.realtime.Publish("message.status_changed", result.MessageKey, messageToDTO(&result))
		}
		s.mu.Unlock()
		return result, nil
	}

	latest, err := s.listMessages(context.Background(), []dataaccess.QueryCondition{
		dataaccess.Eq("conversation_key", conversation.ConversationKey),
		dataaccess.Order("seq", dataaccess.OrderTypeDesc),
		dataaccess.Limit(1),
	}, dataaccess.OrderTypeDesc, 1)
	if err != nil {
		s.mu.Unlock()
		return model.Message{}, err
	}
	var seq int64
	if len(latest) != 0 {
		seq = latest[0].Seq
	}
	seq++
	sourceKeyPtr := (*string)(nil)
	if sourceKey != "" {
		sourceKeyPtr = &sourceKey
	}
	message := &model.Message{
		MessageKey: "msg_" + idutil.GenShortId(), Seq: seq, ConversationKey: conversation.ConversationKey, CustomerKey: conversation.CustomerKey,
		SenderRole: string(req.Sender.Role), SenderID: req.Sender.ID, SourceAppKey: req.Source.AppID, SourceExternalID: req.Source.ExternalID,
		SourceSentAt: req.Source.SentAt, SourceDedupKey: sourceKeyPtr, Type: string(req.Type), Text: req.Text, Media: messageMediaToModel(req.Content), Status: messageStatusForCreate(req.Sender.Role, req.Status),
	}
	if err := s.accessor.Save(context.Background(), message); err != nil {
		s.mu.Unlock()
		return model.Message{}, err
	}
	result := *message
	observer := s.customerObserver
	if publish {
		s.realtime.Publish("message.created", result.MessageKey, messageToDTO(&result))
	}
	s.mu.Unlock()
	if result.SenderRole == string(gatewaydto.MessageSenderRoleCustomer) && observer != nil {
		go observer(result)
	}
	return result, nil
}

func (s *MessageService) SetCustomerObserver(fn func(model.Message)) {
	s.mu.Lock()
	s.customerObserver = fn
	s.mu.Unlock()
}

func (s *MessageService) CreateAIReply(_ context.Context, conversationID, employeeID, text string) (gatewaydto.Message, error) {
	employeeID = strings.TrimSpace(employeeID)
	text = strings.TrimSpace(text)
	if employeeID == "" {
		return gatewaydto.Message{}, badRequest("employee id is required")
	}
	if text == "" {
		return gatewaydto.Message{}, badRequest("text is required")
	}
	message, err := s.Create(context.Background(), gatewaydto.CreateMessageRequest{
		ConversationKey: conversationID,
		Sender:          gatewaydto.MessageSender{Role: gatewaydto.MessageSenderRoleAI, ID: employeeID},
		Type:            gatewaydto.MessageTypeText,
		Text:            text,
	}, true)
	return messageToDTOResult(message, err)
}

func (s *MessageService) RecentMessages(_ context.Context, conversationID string, limit int) ([]gatewaydto.Message, error) {
	if _, err := s.conversations.Get(context.Background(), conversationID); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	messages := s.List(context.Background(), conversationID, 0, 0, limit)
	items := make([]gatewaydto.Message, 0, len(messages))
	for i := range messages {
		items = append(items, messageToDTO(&messages[i]))
	}
	return items, nil
}

func (s *MessageService) CreateChannelMessage(_ context.Context, appID string, req gatewaydto.CreateMessageRequest) (gatewaydto.Message, error) {
	if req.Source.ExternalID == "" {
		return gatewaydto.Message{}, badRequest("source.externalId is required")
	}
	if req.Source.SentAt == nil {
		return gatewaydto.Message{}, badRequest("source.sentAt is required")
	}
	switch req.Sender.Role {
	case gatewaydto.MessageSenderRoleAgent:
		if req.Sender.ID == "" {
			return gatewaydto.Message{}, badRequest("sender.id is required")
		}
		req.Status = gatewaydto.MessageStatusSent
	case gatewaydto.MessageSenderRoleCustomer:
		if req.CustomerID != "" && req.Sender.ID != "" && req.CustomerID != req.Sender.ID {
			return gatewaydto.Message{}, badRequest("customerId does not match sender.id")
		}
	default:
		return gatewaydto.Message{}, forbidden("business channels can only send customer or agent messages")
	}
	req.Source.AppID = appID
	conversation, err := s.conversations.Get(context.Background(), req.ConversationKey)
	if err != nil {
		return gatewaydto.Message{}, err
	}
	if conversation.AppKey != appID {
		return gatewaydto.Message{}, notFound("conversation %s not found", req.ConversationKey)
	}
	if req.CustomerID == "" {
		req.CustomerID = conversation.CustomerKey
	}
	if req.Sender.Role == gatewaydto.MessageSenderRoleCustomer && (req.CustomerID != conversation.CustomerKey || req.Sender.ID != conversation.CustomerKey) {
		return gatewaydto.Message{}, forbidden("sender must be the conversation customer")
	}
	return messageToDTOResult(s.Create(context.Background(), req, true))
}

func (s *MessageService) CreateMessage(_ context.Context, appID string, req gatewaydto.CreateMessageRequest) (model.Message, error) {
	if appID == "" {
		return model.Message{}, forbidden("channel app is required")
	}
	if req.Source.ExternalID == "" {
		return model.Message{}, badRequest("source.externalId is required")
	}
	if req.Source.SentAt == nil {
		return model.Message{}, badRequest("source.sentAt is required")
	}
	switch req.Sender.Role {
	case gatewaydto.MessageSenderRoleAgent:
		if req.Sender.ID == "" {
			return model.Message{}, badRequest("sender.id is required")
		}
		req.Status = gatewaydto.MessageStatusSent
	case gatewaydto.MessageSenderRoleCustomer:
		if req.CustomerID != "" && req.Sender.ID != "" && req.CustomerID != req.Sender.ID {
			return model.Message{}, badRequest("customerId does not match sender.id")
		}
	default:
		return model.Message{}, forbidden("business channels can only send customer or agent messages")
	}
	req.Source.AppID = appID
	conversation, err := s.conversations.Get(context.Background(), req.ConversationKey)
	if err != nil {
		return model.Message{}, err
	}
	if conversation.AppKey != appID {
		return model.Message{}, notFound("conversation %s not found", req.ConversationKey)
	}
	if req.CustomerID == "" {
		req.CustomerID = conversation.CustomerKey
	}
	if req.Sender.Role == gatewaydto.MessageSenderRoleCustomer && (req.CustomerID != conversation.CustomerKey || req.Sender.ID != conversation.CustomerKey) {
		return model.Message{}, forbidden("sender must be the conversation customer")
	}
	return s.Create(context.Background(), req, false)
}

func (s *MessageService) CreateRelayMessage(_ context.Context, relayID string, req gatewaydto.CreateMessageRequest) (gatewaydto.Message, error) {
	if relayID == "" {
		return gatewaydto.Message{}, forbidden("relay identity is required")
	}
	req.Sender = gatewaydto.MessageSender{Role: gatewaydto.MessageSenderRoleAgent, ID: relayID}
	req.Source = gatewaydto.MessageSource{}
	req.CustomerID = ""
	req.Status = gatewaydto.MessageStatusSending
	return messageToDTOResult(s.Create(context.Background(), req, true))
}

func (s *MessageService) ChannelEventAllowed(appID string, event RealtimeEvent) bool {
	if event.Type == "history.request.created" {
		request, ok := event.Data.(gatewaydto.HistoryRequest)
		return ok && request.AppID == appID
	}
	if event.Type != "message.created" {
		return false
	}
	message, ok := event.Data.(gatewaydto.Message)
	if !ok {
		return false
	}
	conversation, err := s.conversations.Get(context.Background(), message.ConversationKey)
	return err == nil && conversation.AppKey == appID
}

func (s *MessageService) RelayEventAllowed(event RealtimeEvent) bool {
	switch event.Type {
	case "conversation.created":
		return true
	case "message.created", "message.created_batch", "message.status_changed":
		switch data := event.Data.(type) {
		case gatewaydto.Message:
			switch data.Sender.Role {
			case gatewaydto.MessageSenderRoleCustomer, gatewaydto.MessageSenderRoleAgent, gatewaydto.MessageSenderRoleAI:
				return true
			default:
				return false
			}
		case []gatewaydto.Message:
			for _, message := range data {
				switch message.Sender.Role {
				case gatewaydto.MessageSenderRoleCustomer, gatewaydto.MessageSenderRoleAgent, gatewaydto.MessageSenderRoleAI:
					return true
				default:
					return false
				}
			}
		}
		return false
	default:
		return false
	}
}

func (s *MessageService) RelayHistory(_ context.Context, conversationID string, filter gatewaydto.MessageHistoryFilter) (gatewaydto.MessageHistoryPage, error) {
	if filter.Order == "" {
		filter.Order = "desc"
	}
	if filter.Order != "asc" && filter.Order != "desc" {
		return gatewaydto.MessageHistoryPage{}, badRequest("order must be asc or desc")
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.AfterSeq > 0 && filter.BeforeSeq > 0 && filter.AfterSeq >= filter.BeforeSeq {
		return gatewaydto.MessageHistoryPage{}, badRequest("afterSeq must be less than beforeSeq")
	}
	if _, err := s.conversations.Get(context.Background(), conversationID); err != nil {
		return gatewaydto.MessageHistoryPage{}, err
	}
	return s.historyPage(context.Background(), conversationID, filter)
}

func messageStatusForCreate(role gatewaydto.MessageSenderRole, requestedStatus gatewaydto.MessageStatus) string {
	if (role == gatewaydto.MessageSenderRoleCustomer || role == gatewaydto.MessageSenderRoleAgent) && requestedStatus.Valid() {
		return string(requestedStatus)
	}
	return string(gatewaydto.MessageStatusReceived)
}

func (s *MessageService) UpdateStatus(_ context.Context, id string, req gatewaydto.UpdateMessageStatusRequest) (model.Message, error) {
	if !req.Status.Valid() {
		return model.Message{}, badRequest("unsupported message status %s", req.Status)
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	message, err := s.Get(context.Background(), id)
	if err != nil {
		return model.Message{}, notFound("message %s not found", id)
	}
	message.Status = string(req.Status)
	message.UpdateAt = now
	if err := s.accessor.Save(context.Background(), &message); err != nil {
		return model.Message{}, err
	}
	result := message
	s.realtime.Publish("message.status_changed", id, messageToDTO(&result))
	return result, nil
}

func (s *MessageService) UpdateChannelMessageStatus(_ context.Context, appID, id string, req gatewaydto.UpdateMessageStatusRequest) (gatewaydto.Message, error) {
	if !req.Status.Valid() {
		return gatewaydto.Message{}, badRequest("unsupported message status %s", req.Status)
	}
	message, err := s.Get(context.Background(), id)
	if err != nil {
		return gatewaydto.Message{}, err
	}
	conversation, err := s.conversations.Get(context.Background(), message.ConversationKey)
	if err != nil {
		return gatewaydto.Message{}, err
	}
	if conversation.AppKey != appID {
		return gatewaydto.Message{}, forbidden("message not found")
	}
	return messageToDTOResult(s.UpdateStatus(context.Background(), id, req))
}

func (s *MessageService) Get(_ context.Context, id string) (model.Message, error) {
	items, err := s.listMessages(context.Background(), []dataaccess.QueryCondition{
		dataaccess.Eq("message_key", id),
		dataaccess.Limit(1),
	}, dataaccess.OrderTypeDesc, 1)
	if err != nil {
		return model.Message{}, err
	}
	if len(items) == 0 {
		return model.Message{}, notFound("message %s not found", id)
	}
	return items[0], nil
}

func (s *MessageService) RetryMessage(_ context.Context, id string) (model.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	message, err := s.Get(context.Background(), id)
	if err != nil {
		return model.Message{}, notFound("message %s not found", id)
	}
	if message.SenderRole != string(gatewaydto.MessageSenderRoleAgent) {
		return model.Message{}, forbidden("only agent messages can be retried")
	}
	if message.Status != string(gatewaydto.MessageStatusFailed) {
		return model.Message{}, badRequest("only failed messages can be retried")
	}
	message.Status = string(gatewaydto.MessageStatusReceived)
	message.UpdateAt = time.Now().UTC()
	if err := s.accessor.Save(context.Background(), &message); err != nil {
		return model.Message{}, err
	}
	s.realtime.Publish("message.created", message.MessageKey, messageToDTO(&message))
	return message, nil
}

func (s *MessageService) Latest(_ context.Context, conversationID string) (model.Message, error) {
	items, err := s.listMessages(context.Background(), []dataaccess.QueryCondition{
		dataaccess.Eq("conversation_key", conversationID),
		dataaccess.Order("seq", dataaccess.OrderTypeDesc),
		dataaccess.Limit(1),
	}, dataaccess.OrderTypeDesc, 1)
	if err != nil {
		return model.Message{}, err
	}
	if len(items) == 0 {
		return model.Message{}, badRequest("conversation has no message to forward")
	}
	return items[len(items)-1], nil
}

func (s *MessageService) ConversationStats(_ context.Context, conversationID string) (ConversationMessageStats, error) {
	messages, err := s.listMessages(context.Background(), []dataaccess.QueryCondition{
		dataaccess.Eq("conversation_key", conversationID),
	}, dataaccess.OrderTypeAsc, 0)
	if err != nil {
		return ConversationMessageStats{}, err
	}
	stats := ConversationMessageStats{ReplyStatus: "unreplied"}
	for _, message := range messages {
		if message.SenderRole == "customer" && message.Status != "read" {
			stats.UnreadCount++
		}
	}
	if len(messages) > 0 {
		latest := messages[len(messages)-1]
		if latest.SenderRole == "agent" || latest.SenderRole == "ai" {
			stats.ReplyStatus = "replied"
		}
	}
	return stats, nil
}

func (s *MessageService) LastReceivedActivity(_ context.Context, conversationID string) (time.Time, error) {
	messages, err := s.listMessages(context.Background(), []dataaccess.QueryCondition{
		dataaccess.Eq("conversation_key", conversationID),
	}, dataaccess.OrderTypeAsc, 0)
	if err != nil {
		return time.Time{}, err
	}
	var latest time.Time
	for _, message := range messages {
		if createdAt := message.CreatedAt(); createdAt.After(latest) {
			latest = createdAt
		}
	}
	if latest.IsZero() {
		return time.Time{}, badRequest("conversation has no activity")
	}
	return latest, nil
}

func (s *MessageService) MarkConversationRead(_ context.Context, conversationID string) (int, error) {
	messages, err := s.listMessages(context.Background(), []dataaccess.QueryCondition{
		dataaccess.Eq("conversation_key", conversationID),
	}, dataaccess.OrderTypeAsc, 0)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	markedCount := 0
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, message := range messages {
		if message.SenderRole != "customer" || message.Status == "read" {
			continue
		}
		message.Status = "read"
		message.UpdateAt = now
		if err := s.accessor.Save(context.Background(), &message); err != nil {
			return markedCount, err
		}
		markedCount++
		s.realtime.Publish("message.status_changed", message.MessageKey, messageToDTO(&message))
	}
	return markedCount, nil
}

func (s *MessageService) List(_ context.Context, conversationID string, afterSeq, beforeSeq int64, limit int) []model.Message {
	messages, err := s.listMessages(context.Background(), s.listConditions(conversationID, afterSeq, beforeSeq), dataaccess.OrderTypeDesc, limit)
	if err != nil {
		return []model.Message{}
	}
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}
	return messages
}

func (s *MessageService) History(_ context.Context, appID, conversationID string, filter gatewaydto.MessageHistoryFilter) (gatewaydto.MessageHistoryPage, error) {
	if filter.Order == "" {
		filter.Order = "desc"
	}
	if filter.Order != "asc" && filter.Order != "desc" {
		return gatewaydto.MessageHistoryPage{}, badRequest("order must be asc or desc")
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.AfterSeq > 0 && filter.BeforeSeq > 0 && filter.AfterSeq >= filter.BeforeSeq {
		return gatewaydto.MessageHistoryPage{}, badRequest("afterSeq must be less than beforeSeq")
	}
	if _, err := s.apps.Get(context.Background(), appID); err != nil {
		return gatewaydto.MessageHistoryPage{}, err
	}
	conversation, err := s.conversations.Get(context.Background(), conversationID)
	if err != nil {
		return gatewaydto.MessageHistoryPage{}, err
	}
	if conversation.AppKey != appID {
		return gatewaydto.MessageHistoryPage{}, notFound("conversation %s not found", conversationID)
	}
	return s.historyPage(context.Background(), conversationID, filter)
}

func (s *MessageService) HistoryForApp(ctx context.Context, appID, conversationID string, filter gatewaydto.MessageHistoryFilter) (any, error) {
	page, err := s.History(ctx, appID, conversationID, filter)
	if err != nil {
		return nil, err
	}
	return page, nil
}

func (s *MessageService) CreateHistoryRequest(ctx context.Context, conversationID string) (gatewaydto.HistoryRequest, error) {
	conversation, err := s.conversations.Get(ctx, conversationID)
	if err != nil {
		return gatewaydto.HistoryRequest{}, err
	}
	customerExternalID, err := s.conversations.GetCustomerExternalID(ctx, conversation.CustomerKey)
	if err != nil {
		return gatewaydto.HistoryRequest{}, err
	}
	request := gatewaydto.HistoryRequest{
		RequestID:          "hsr_" + idutil.GenShortId(),
		AppID:              conversation.AppKey,
		ConversationID:     conversation.ConversationKey,
		CustomerExternalID: customerExternalID,
		Status:             gatewaydto.HistoryRequestStatusPending,
		CreatedAt:          time.Now().UTC(),
	}
	stored := &model.HistoryRequest{
		RequestKey:         request.RequestID,
		AppKey:             request.AppID,
		ConversationKey:    request.ConversationID,
		CustomerExternalID: request.CustomerExternalID,
		Status:             string(request.Status),
	}
	if err := s.historyAccessor.Save(ctx, stored); err != nil {
		return gatewaydto.HistoryRequest{}, err
	}
	s.realtime.Publish("history.request.created", request.RequestID, request)
	return request, nil
}

func (s *MessageService) PendingHistoryRequest(ctx context.Context, appID string) (gatewaydto.HistoryRequest, bool, error) {
	if _, err := s.apps.Get(ctx, appID); err != nil {
		return gatewaydto.HistoryRequest{}, false, err
	}
	stored, err := s.nextPendingHistoryRequest(ctx, appID)
	if err != nil {
		return gatewaydto.HistoryRequest{}, false, err
	}
	if stored == nil {
		return gatewaydto.HistoryRequest{}, false, nil
	}
	return historyRequestToDTO(stored), true, nil
}

func (s *MessageService) CompleteHistoryRequest(ctx context.Context, appID string, req gatewaydto.CompleteHistoryRequestInput) (gatewaydto.HistoryRequest, error) {
	if _, err := s.apps.Get(ctx, appID); err != nil {
		return gatewaydto.HistoryRequest{}, err
	}
	stored, err := s.getHistoryRequest(ctx, appID, req.RequestID)
	if err != nil {
		return gatewaydto.HistoryRequest{}, notFound("history request %s not found", req.RequestID)
	}
	stored.Status = string(gatewaydto.HistoryRequestStatusCompleted)
	now := time.Now().UTC()
	stored.CompletedAt = &now
	if err := s.historyAccessor.Save(ctx, stored); err != nil {
		return gatewaydto.HistoryRequest{}, err
	}
	return historyRequestToDTO(stored), nil
}

func (s *MessageService) FailHistoryRequest(ctx context.Context, appID string, req gatewaydto.CompleteHistoryRequestInput) (gatewaydto.HistoryRequest, error) {
	if _, err := s.apps.Get(ctx, appID); err != nil {
		return gatewaydto.HistoryRequest{}, err
	}
	stored, err := s.getHistoryRequest(ctx, appID, req.RequestID)
	if err != nil {
		return gatewaydto.HistoryRequest{}, notFound("history request %s not found", req.RequestID)
	}
	stored.Status = string(gatewaydto.HistoryRequestStatusFailed)
	now := time.Now().UTC()
	stored.CompletedAt = &now
	if err := s.historyAccessor.Save(ctx, stored); err != nil {
		return gatewaydto.HistoryRequest{}, err
	}
	return historyRequestToDTO(stored), nil
}

func (s *MessageService) ReverseHistory(ctx context.Context, conversationID string, req gatewaydto.BatchCreateMessagesRequest, filter gatewaydto.MessageHistoryFilter) (gatewaydto.MessageHistoryPage, int, error) {
	conversation, err := s.conversations.Get(ctx, conversationID)
	if err != nil {
		return gatewaydto.MessageHistoryPage{}, 0, err
	}
	appID := conversation.AppKey

	before, err := s.accessor.Count(ctx, dataaccess.Eq("conversation_key", conversationID))
	if err != nil {
		return gatewaydto.MessageHistoryPage{}, 0, err
	}
	if len(req.Items) > 0 {
		if _, err := s.CreateChannelMessages(ctx, appID, req); err != nil {
			return gatewaydto.MessageHistoryPage{}, 0, err
		}
	}
	after, err := s.accessor.Count(ctx, dataaccess.Eq("conversation_key", conversationID))
	if err != nil {
		return gatewaydto.MessageHistoryPage{}, 0, err
	}
	page, err := s.History(ctx, appID, conversationID, filter)
	if err != nil {
		return gatewaydto.MessageHistoryPage{}, 0, err
	}
	createdCount := int(after - before)
	if createdCount < 0 {
		createdCount = 0
	}
	return page, createdCount, nil
}

func (s *MessageService) Offset(_ context.Context, appID, conversationID string) (gatewaydto.MessageOffset, error) {
	conversation, err := s.conversations.Get(context.Background(), conversationID)
	if err != nil {
		return gatewaydto.MessageOffset{}, err
	}
	if conversation.AppKey != appID {
		return gatewaydto.MessageOffset{}, notFound("conversation %s not found", conversationID)
	}
	message, err := s.Latest(context.Background(), conversationID)
	if err != nil {
		return gatewaydto.MessageOffset{ConversationKey: conversationID}, nil
	}
	return gatewaydto.MessageOffset{
		ConversationKey:  conversationID,
		LatestSeq:        message.Seq,
		LatestMessageID:  message.MessageKey,
		LatestExternalID: message.SourceExternalID,
		LatestSentAt:     message.SourceSentAt,
	}, nil
}

func (s *MessageService) CreateChannelMessages(ctx context.Context, appID string, req gatewaydto.BatchCreateMessagesRequest) (gatewaydto.BatchCreateMessagesResult, error) {
	if len(req.Items) == 0 {
		return gatewaydto.BatchCreateMessagesResult{}, nil
	}
	models := make([]model.Message, 0, len(req.Items))
	if err := s.accessor.Transaction(ctx, func(ctx context.Context) error {
		models = models[:0]
		for _, item := range req.Items {
			message, err := s.CreateMessage(ctx, appID, item)
			if err != nil {
				return err
			}
			models = append(models, message)
		}
		return nil
	}); err != nil {
		return gatewaydto.BatchCreateMessagesResult{}, err
	}
	items := make([]gatewaydto.Message, 0, len(models))
	for i := range models {
		items = append(items, messageToDTO(&models[i]))
	}
	if len(items) > 0 {
		s.realtime.Publish("message.created_batch", "batch", items)
	}
	return gatewaydto.BatchCreateMessagesResult{Items: items}, nil
}

func (s *MessageService) listConditions(conversationID string, afterSeq, beforeSeq int64) []dataaccess.QueryCondition {
	conditions := []dataaccess.QueryCondition{dataaccess.Eq("conversation_key", conversationID)}
	if afterSeq > 0 {
		conditions = append(conditions, dataaccess.Gt("seq", afterSeq))
	}
	if beforeSeq > 0 {
		conditions = append(conditions, dataaccess.Lt("seq", beforeSeq))
	}
	return conditions
}

func (s *MessageService) listMessages(ctx context.Context, conditions []dataaccess.QueryCondition, order dataaccess.OrderType, fetchLimit int) ([]model.Message, error) {
	conditions = append(conditions, dataaccess.Order("seq", order))
	if fetchLimit > 0 {
		conditions = append(conditions, dataaccess.Limit(fetchLimit))
	}
	var found []*model.Message
	if err := s.accessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	messages := make([]model.Message, 0, len(found))
	for _, item := range found {
		messages = append(messages, *item)
	}
	return messages, nil
}

func (s *MessageService) historyPage(ctx context.Context, conversationID string, filter gatewaydto.MessageHistoryFilter) (gatewaydto.MessageHistoryPage, error) {
	order := dataaccess.OrderTypeDesc
	if filter.Order == "asc" {
		order = dataaccess.OrderTypeAsc
	}
	messages, err := s.listMessages(ctx, s.listConditions(conversationID, filter.AfterSeq, filter.BeforeSeq), order, filter.Limit+1)
	if err != nil {
		return gatewaydto.MessageHistoryPage{}, err
	}
	hasMore := len(messages) > filter.Limit
	if hasMore {
		messages = messages[:filter.Limit]
	}
	page := gatewaydto.MessageHistoryPage{Items: make([]gatewaydto.Message, 0, len(messages)), HasMore: hasMore}
	for i := range messages {
		page.Items = append(page.Items, messageToDTO(&messages[i]))
	}
	if len(messages) > 0 {
		last := messages[len(messages)-1].Seq
		if order == dataaccess.OrderTypeAsc {
			page.NextAfterSeq = last
		} else {
			page.NextBeforeSeq = last
		}
	}
	return page, nil
}

func (s *MessageService) nextPendingHistoryRequest(ctx context.Context, appID string) (*model.HistoryRequest, error) {
	found, err := s.getHistoryRequests(ctx, appID, string(gatewaydto.HistoryRequestStatusPending), 1)
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	return found[0], nil
}

func (s *MessageService) getHistoryRequest(ctx context.Context, appID, requestID string) (*model.HistoryRequest, error) {
	conditions := []dataaccess.QueryCondition{
		dataaccess.Eq("app_key", appID),
		dataaccess.Eq("request_key", requestID),
		dataaccess.Limit(1),
	}
	items, err := s.getHistoryRequestItems(ctx, conditions...)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, dataaccess.ErrDataNotFound
	}
	return items[0], nil
}

func (s *MessageService) getHistoryRequests(ctx context.Context, appID, status string, limit int) ([]*model.HistoryRequest, error) {
	conditions := []dataaccess.QueryCondition{dataaccess.Eq("app_key", appID)}
	if status != "" {
		conditions = append(conditions, dataaccess.Eq("status", status))
	}
	conditions = append(conditions, dataaccess.Order("create_at", dataaccess.OrderTypeDesc))
	if limit > 0 {
		conditions = append(conditions, dataaccess.Limit(limit))
	}
	var found []*model.HistoryRequest
	if err := s.historyAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	return found, nil
}

func (s *MessageService) getHistoryRequestItems(ctx context.Context, conditions ...dataaccess.QueryCondition) ([]*model.HistoryRequest, error) {
	var found []*model.HistoryRequest
	if err := s.historyAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	return found, nil
}

func historyRequestToDTO(item *model.HistoryRequest) gatewaydto.HistoryRequest {
	return gatewaydto.HistoryRequest{
		RequestID:          item.RequestKey,
		AppID:              item.AppKey,
		ConversationID:     item.ConversationKey,
		CustomerExternalID: item.CustomerExternalID,
		Status:             gatewaydto.HistoryRequestStatus(item.Status),
		CreatedAt:          item.CreatedAt,
		CompletedAt:        item.CompletedAt,
	}
}
