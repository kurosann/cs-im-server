package service

import (
	"context"
	"sort"
	"sync"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

	"cs-im-server/pkg/seqquery"
)

type MessageService struct {
	mu              sync.RWMutex
	accessor        dataaccess.DataAccessor[*Message]
	messages        map[string]*Message
	sourceKeys      map[string]string
	conversationSeq map[string]int64
	apps            *AppService
	conversations   *ConversationService
	realtime        *RealtimeService
}

type CreateMessageRequest struct {
	ConversationKey string        `json:"conversationId"`
	CustomerID      string        `json:"customerId"`
	Sender          MessageSender `json:"sender"`
	Source          MessageSource `json:"source"`
	Type            string        `json:"type"`
	Text            string        `json:"text"`
}

type UpdateMessageStatusRequest struct {
	Status string `json:"status"`
}

type MessageHistoryFilter = seqquery.Filter

type MessageHistoryPage struct {
	Items         []Message `json:"items"`
	NextAfterSeq  int64     `json:"nextAfterSeq"`
	NextBeforeSeq int64     `json:"nextBeforeSeq"`
	HasMore       bool      `json:"hasMore"`
}

var messageStatuses = map[string]bool{
	"received": true, "sent": true, "delivered": true, "read": true, "failed": true,
}

func NewMessageService(apps *AppService, conversations *ConversationService, realtime *RealtimeService) *MessageService {
	return &MessageService{
		messages:        make(map[string]*Message),
		sourceKeys:      make(map[string]string),
		conversationSeq: make(map[string]int64),
		apps:            apps, conversations: conversations, realtime: realtime,
	}
}

func messageSourceDedupKey(appID, externalID string) string {
	if appID == "" || externalID == "" {
		return ""
	}
	return appID + ":" + externalID
}

func (s *MessageService) Create(_ context.Context, req CreateMessageRequest) (Message, error) {
	if req.ConversationKey == "" {
		return Message{}, badRequest("conversationId is required")
	}
	if req.Sender.Role == "" {
		return Message{}, badRequest("sender.role is required")
	}
	if req.Sender.ID == "" {
		return Message{}, badRequest("sender.id is required")
	}
	if req.Type == "" {
		req.Type = "text"
	}
	if req.Type == "text" && req.Text == "" {
		return Message{}, badRequest("text is required")
	}
	conversation, err := s.conversations.Get(context.Background(), req.ConversationKey)
	if err != nil {
		return Message{}, err
	}
	if req.CustomerID == "" {
		req.CustomerID = conversation.CustomerKey
	}
	if req.CustomerID != conversation.CustomerKey {
		return Message{}, badRequest("customer %s does not belong to conversation", req.CustomerID)
	}
	if conversation.Status != "open" {
		return Message{}, badRequest("conversation %s is closed", conversation.ConversationKey)
	}

	sourceKey := messageSourceDedupKey(req.Source.AppID, req.Source.ExternalID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if sourceKey != "" {
		if messageID, ok := s.sourceKeys[sourceKey]; ok {
			return *s.messages[messageID], nil
		}
	}
	seq := s.conversationSeq[conversation.ConversationKey] + 1
	sourceKeyPtr := (*string)(nil)
	if sourceKey != "" {
		sourceKeyPtr = &sourceKey
	}
	message := &Message{
		MessageKey: "msg_" + idutil.GenShortId(), Seq: seq, ConversationKey: conversation.ConversationKey, CustomerKey: conversation.CustomerKey,
		SenderRole: req.Sender.Role, SenderID: req.Sender.ID, SourceAppKey: req.Source.AppID, SourceExternalID: req.Source.ExternalID,
		SourceSentAt: req.Source.SentAt, SourceDedupKey: sourceKeyPtr, Type: req.Type, Text: req.Text, Status: "received",
	}
	if err := s.accessor.Save(context.Background(), message); err != nil {
		return Message{}, err
	}
	s.messages[message.MessageKey] = message
	if sourceKey != "" {
		s.sourceKeys[sourceKey] = message.MessageKey
	}
	s.conversationSeq[conversation.ConversationKey] = seq
	result := *message
	s.realtime.Publish("message.created", message.MessageKey, result)
	return result, nil
}

func (s *MessageService) CreateChannelMessage(_ context.Context, appID string, req CreateMessageRequest) (Message, error) {
	if req.Source.ExternalID == "" {
		return Message{}, badRequest("source.externalId is required")
	}
	if req.Source.SentAt == nil {
		return Message{}, badRequest("source.sentAt is required")
	}
	if req.Sender.Role != "customer" {
		return Message{}, forbidden("business channels can only send customer messages")
	}
	if req.CustomerID != "" && req.Sender.ID != "" && req.CustomerID != req.Sender.ID {
		return Message{}, badRequest("customerId does not match sender.id")
	}
	req.Source.AppID = appID
	conversation, err := s.conversations.Get(context.Background(), req.ConversationKey)
	if err != nil {
		return Message{}, err
	}
	if conversation.AppKey != appID {
		return Message{}, notFound("conversation %s not found", req.ConversationKey)
	}
	if req.CustomerID == "" {
		req.CustomerID = conversation.CustomerKey
	}
	if req.CustomerID != conversation.CustomerKey || req.Sender.ID != conversation.CustomerKey {
		return Message{}, forbidden("sender must be the conversation customer")
	}
	return s.Create(context.Background(), req)
}

func (s *MessageService) CreateRelayMessage(_ context.Context, relayID string, req CreateMessageRequest) (Message, error) {
	if relayID == "" {
		return Message{}, forbidden("relay identity is required")
	}
	req.Sender = MessageSender{Role: "agent", ID: relayID}
	req.Source = MessageSource{}
	req.CustomerID = ""
	return s.Create(context.Background(), req)
}

func (s *MessageService) ChannelEventAllowed(appID string, event RealtimeEvent) bool {
	if event.Type != "message.created" {
		return false
	}
	message, ok := event.Data.(Message)
	if !ok || message.SenderRole == "customer" {
		return false
	}
	conversation, err := s.conversations.Get(context.Background(), message.ConversationKey)
	return err == nil && conversation.AppKey == appID
}

func (s *MessageService) RelayEventAllowed(event RealtimeEvent) bool {
	if event.Type != "message.created" {
		return false
	}
	message, ok := event.Data.(Message)
	return ok && message.SenderRole == "customer"
}

func (s *MessageService) RelayHistory(_ context.Context, conversationID string, filter MessageHistoryFilter) (MessageHistoryPage, error) {
	if filter.Order == "" {
		filter.Order = "desc"
	}
	if filter.Order != "asc" && filter.Order != "desc" {
		return MessageHistoryPage{}, badRequest("order must be asc or desc")
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.AfterSeq > 0 && filter.BeforeSeq > 0 && filter.AfterSeq >= filter.BeforeSeq {
		return MessageHistoryPage{}, badRequest("afterSeq must be less than beforeSeq")
	}
	if _, err := s.conversations.Get(context.Background(), conversationID); err != nil {
		return MessageHistoryPage{}, err
	}
	messages := s.List(context.Background(), conversationID, filter.AfterSeq, filter.BeforeSeq, 0)
	if filter.Order == "desc" {
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}
	page := MessageHistoryPage{Items: make([]Message, 0, filter.Limit)}
	hasMore := len(messages) > filter.Limit
	if hasMore {
		messages = messages[:filter.Limit]
	}
	page.Items = messages
	page.HasMore = hasMore
	if len(messages) > 0 {
		last := messages[len(messages)-1].Seq
		if filter.Order == "asc" {
			page.NextAfterSeq = last
		} else {
			page.NextBeforeSeq = last
		}
	}
	return page, nil
}

func (s *MessageService) UpdateStatus(_ context.Context, id string, req UpdateMessageStatusRequest) (Message, error) {
	if !messageStatuses[req.Status] {
		return Message{}, badRequest("unsupported message status %s", req.Status)
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	message, ok := s.messages[id]
	if !ok {
		return Message{}, notFound("message %s not found", id)
	}
	message.Status = req.Status
	message.UpdateAt = now
	if err := s.accessor.Save(context.Background(), message); err != nil {
		return Message{}, err
	}
	result := *message
	s.realtime.Publish("message.status_changed", id, result)
	return result, nil
}

func (s *MessageService) Get(_ context.Context, id string) (Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	message, ok := s.messages[id]
	if !ok {
		return Message{}, notFound("message %s not found", id)
	}
	return *message, nil
}

func (s *MessageService) Latest(_ context.Context, conversationID string) (Message, error) {
	items := s.List(context.Background(), conversationID, 0, 0, 1)
	if len(items) == 0 {
		return Message{}, badRequest("conversation has no message to forward")
	}
	return items[len(items)-1], nil
}

func (s *MessageService) List(_ context.Context, conversationID string, afterSeq, beforeSeq int64, limit int) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	messages := make([]Message, 0, len(s.messages))
	for _, message := range s.messages {
		if message.ConversationKey != conversationID {
			continue
		}
		if afterSeq > 0 && message.Seq <= afterSeq {
			continue
		}
		if beforeSeq > 0 && message.Seq >= beforeSeq {
			continue
		}
		messages = append(messages, *message)
	}
	sort.Slice(messages, func(i, j int) bool { return messages[i].Seq < messages[j].Seq })
	if limit > 0 && len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	return messages
}

func (s *MessageService) History(_ context.Context, appID, conversationID string, filter MessageHistoryFilter) (MessageHistoryPage, error) {
	if filter.Order == "" {
		filter.Order = "desc"
	}
	if filter.Order != "asc" && filter.Order != "desc" {
		return MessageHistoryPage{}, badRequest("order must be asc or desc")
	}
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.AfterSeq > 0 && filter.BeforeSeq > 0 && filter.AfterSeq >= filter.BeforeSeq {
		return MessageHistoryPage{}, badRequest("afterSeq must be less than beforeSeq")
	}
	if _, err := s.apps.Get(context.Background(), appID); err != nil {
		return MessageHistoryPage{}, err
	}
	conversation, err := s.conversations.Get(context.Background(), conversationID)
	if err != nil {
		return MessageHistoryPage{}, err
	}
	if conversation.AppKey != appID {
		return MessageHistoryPage{}, notFound("conversation %s not found", conversationID)
	}
	messages := s.List(context.Background(), conversationID, filter.AfterSeq, filter.BeforeSeq, 0)
	if filter.Order == "desc" {
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}
	page := MessageHistoryPage{Items: make([]Message, 0, filter.Limit)}
	hasMore := len(messages) > filter.Limit
	if hasMore {
		messages = messages[:filter.Limit]
	}
	page.Items = messages
	page.HasMore = hasMore
	if len(messages) > 0 {
		last := messages[len(messages)-1].Seq
		if filter.Order == "asc" {
			page.NextAfterSeq = last
			page.NextBeforeSeq = 0
		} else {
			page.NextAfterSeq = 0
			page.NextBeforeSeq = last
		}
	}
	return page, nil
}

func (s *MessageService) HistoryForApp(ctx context.Context, appID, conversationID string, filter MessageHistoryFilter) (any, error) {
	page, err := s.History(ctx, appID, conversationID, filter)
	if err != nil {
		return nil, err
	}
	return page, nil
}

func (s *MessageService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.accessor.List(context.Background())
	if err != nil {
		return err
	}
	for _, message := range items {
		s.messages[message.MessageKey] = message
		if message.SourceDedupKey != nil && *message.SourceDedupKey != "" {
			s.sourceKeys[*message.SourceDedupKey] = message.MessageKey
		}
		if message.Seq > s.conversationSeq[message.ConversationKey] {
			s.conversationSeq[message.ConversationKey] = message.Seq
		}
	}
	return nil
}
