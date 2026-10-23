package service

import (
	"context"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
)

type MessageService struct {
	mu               sync.RWMutex
	loadOnce         sync.Once
	loadErr          error
	accessor         dataaccess.DataAccessor[*Message]
	messages         map[string]*Message
	sourceKeys       map[string]string
	conversationSeq  map[string]int64
	apps             *AppService
	conversations    *ConversationService
	realtime         *RealtimeService
	customerObserver func(Message)
}

var messageStatuses = map[string]bool{
	"received": true, "sent": true, "delivered": true, "read": true, "failed": true,
}

var supportedMessageTypes = map[string]bool{"text": true, "image": true, "file": true, "video": true}

func messageStatusForCreate(role string, requestedStatus string) string {
	if role == "customer" && messageStatuses[requestedStatus] {
		return requestedStatus
	}
	return "received"
}

func validateMessage(req *gatewaydto.CreateMessageRequest) error {
	if req.Type == "" {
		req.Type = "text"
	}
	if !supportedMessageTypes[req.Type] {
		return badRequest("unsupported message type %s", req.Type)
	}
	if req.Type == "text" {
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

func (s *MessageService) Create(_ context.Context, req gatewaydto.CreateMessageRequest) (Message, error) {
	if err := s.ensureLoaded(); err != nil {
		return Message{}, err
	}
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
	if err := validateMessage(&req); err != nil {
		return Message{}, err
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
	if sourceKey != "" {
		if messageID, ok := s.sourceKeys[sourceKey]; ok {
			existing := *s.messages[messageID]
			s.mu.Unlock()
			return existing, nil
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
		SourceSentAt: req.Source.SentAt, SourceDedupKey: sourceKeyPtr, Type: req.Type, Text: req.Text, Media: messageMediaToModel(req.Content), Status: messageStatusForCreate(req.Sender.Role, req.Status),
	}
	if err := s.accessor.Save(context.Background(), message); err != nil {
		s.mu.Unlock()
		return Message{}, err
	}
	s.messages[message.MessageKey] = message
	if sourceKey != "" {
		s.sourceKeys[sourceKey] = message.MessageKey
	}
	s.conversationSeq[conversation.ConversationKey] = seq
	result := *message
	observer := s.customerObserver
	s.realtime.Publish("message.created", message.MessageKey, messageToDTO(&result))
	s.mu.Unlock()
	if result.SenderRole == "customer" && observer != nil {
		go observer(result)
	}
	return result, nil
}

func (s *MessageService) SetCustomerObserver(fn func(Message)) {
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
		Sender:          gatewaydto.MessageSender{Role: "ai", ID: employeeID},
		Type:            "text",
		Text:            text,
	})
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
	if err := s.ensureLoaded(); err != nil {
		return gatewaydto.Message{}, err
	}
	if req.Source.ExternalID == "" {
		return gatewaydto.Message{}, badRequest("source.externalId is required")
	}
	if req.Source.SentAt == nil {
		return gatewaydto.Message{}, badRequest("source.sentAt is required")
	}
	switch req.Sender.Role {
	case "agent":
		if req.Sender.ID == "" {
			return gatewaydto.Message{}, badRequest("sender.id is required")
		}
	case "customer":
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
	if req.Sender.Role == "customer" && (req.CustomerID != conversation.CustomerKey || req.Sender.ID != conversation.CustomerKey) {
		return gatewaydto.Message{}, forbidden("sender must be the conversation customer")
	}
	return messageToDTOResult(s.Create(context.Background(), req))
}

func (s *MessageService) CreateRelayMessage(_ context.Context, relayID string, req gatewaydto.CreateMessageRequest) (gatewaydto.Message, error) {
	if err := s.ensureLoaded(); err != nil {
		return gatewaydto.Message{}, err
	}
	if relayID == "" {
		return gatewaydto.Message{}, forbidden("relay identity is required")
	}
	req.Sender = gatewaydto.MessageSender{Role: "agent", ID: relayID}
	req.Source = gatewaydto.MessageSource{}
	req.CustomerID = ""
	return messageToDTOResult(s.Create(context.Background(), req))
}

func (s *MessageService) ChannelEventAllowed(appID string, event RealtimeEvent) bool {
	if err := s.ensureLoaded(); err != nil {
		return false
	}
	if event.Type != "message.created" {
		return false
	}
	message, ok := event.Data.(gatewaydto.Message)
	if !ok || message.Sender.Role == "customer" {
		return false
	}
	conversation, err := s.conversations.Get(context.Background(), message.ConversationKey)
	return err == nil && conversation.AppKey == appID
}

func (s *MessageService) RelayEventAllowed(event RealtimeEvent) bool {
	if err := s.ensureLoaded(); err != nil {
		return false
	}
	switch event.Type {
	case "conversation.created":
		return true
	case "message.created":
		message, ok := event.Data.(gatewaydto.Message)
		if !ok {
			return false
		}
		switch message.Sender.Role {
		case "customer", "agent", "ai":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func (s *MessageService) RelayHistory(_ context.Context, conversationID string, filter gatewaydto.MessageHistoryFilter) (gatewaydto.MessageHistoryPage, error) {
	if err := s.ensureLoaded(); err != nil {
		return gatewaydto.MessageHistoryPage{}, err
	}
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
	messages := s.List(context.Background(), conversationID, filter.AfterSeq, filter.BeforeSeq, 0)
	if filter.Order == "desc" {
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}
	page := gatewaydto.MessageHistoryPage{Items: make([]gatewaydto.Message, 0, filter.Limit)}
	hasMore := len(messages) > filter.Limit
	if hasMore {
		messages = messages[:filter.Limit]
	}
	page.Items = make([]gatewaydto.Message, 0, len(messages))
	for i := range messages {
		page.Items = append(page.Items, messageToDTO(&messages[i]))
	}
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

func (s *MessageService) UpdateStatus(_ context.Context, id string, req gatewaydto.UpdateMessageStatusRequest) (Message, error) {
	if err := s.ensureLoaded(); err != nil {
		return Message{}, err
	}
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
	s.realtime.Publish("message.status_changed", id, messageToDTO(&result))
	return result, nil
}

func (s *MessageService) Get(_ context.Context, id string) (Message, error) {
	if err := s.ensureLoaded(); err != nil {
		return Message{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	message, ok := s.messages[id]
	if !ok {
		return Message{}, notFound("message %s not found", id)
	}
	return *message, nil
}

func (s *MessageService) Latest(_ context.Context, conversationID string) (Message, error) {
	if err := s.ensureLoaded(); err != nil {
		return Message{}, err
	}
	items := s.List(context.Background(), conversationID, 0, 0, 1)
	if len(items) == 0 {
		return Message{}, badRequest("conversation has no message to forward")
	}
	return items[len(items)-1], nil
}

func (s *MessageService) ConversationStats(_ context.Context, conversationID string) (ConversationMessageStats, error) {
	if err := s.ensureLoaded(); err != nil {
		return ConversationMessageStats{}, err
	}
	messages := s.List(context.Background(), conversationID, 0, 0, 0)
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
	if err := s.ensureLoaded(); err != nil {
		return time.Time{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	var latest time.Time
	for _, message := range s.messages {
		if message.ConversationKey != conversationID {
			continue
		}
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
	if err := s.ensureLoaded(); err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	markedCount := 0
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, message := range s.messages {
		if message.ConversationKey != conversationID || message.SenderRole != "customer" || message.Status == "read" {
			continue
		}
		message.Status = "read"
		message.UpdateAt = now
		if err := s.accessor.Save(context.Background(), message); err != nil {
			return markedCount, err
		}
		markedCount++
		s.realtime.Publish("message.status_changed", message.MessageKey, messageToDTO(message))
	}
	return markedCount, nil
}

func (s *MessageService) List(_ context.Context, conversationID string, afterSeq, beforeSeq int64, limit int) []Message {
	if err := s.ensureLoaded(); err != nil {
		return []Message{}
	}
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

func (s *MessageService) History(_ context.Context, appID, conversationID string, filter gatewaydto.MessageHistoryFilter) (gatewaydto.MessageHistoryPage, error) {
	if err := s.ensureLoaded(); err != nil {
		return gatewaydto.MessageHistoryPage{}, err
	}
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
	messages := s.List(context.Background(), conversationID, filter.AfterSeq, filter.BeforeSeq, 0)
	if filter.Order == "desc" {
		for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
			messages[i], messages[j] = messages[j], messages[i]
		}
	}
	page := gatewaydto.MessageHistoryPage{Items: make([]gatewaydto.Message, 0, filter.Limit)}
	hasMore := len(messages) > filter.Limit
	if hasMore {
		messages = messages[:filter.Limit]
	}
	page.Items = make([]gatewaydto.Message, 0, len(messages))
	for i := range messages {
		page.Items = append(page.Items, messageToDTO(&messages[i]))
	}
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

func (s *MessageService) HistoryForApp(ctx context.Context, appID, conversationID string, filter gatewaydto.MessageHistoryFilter) (any, error) {
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	page, err := s.History(ctx, appID, conversationID, filter)
	if err != nil {
		return nil, err
	}
	return page, nil
}

func (s *MessageService) ReverseHistory(ctx context.Context, conversationID string, req gatewaydto.BatchCreateMessagesRequest, filter gatewaydto.MessageHistoryFilter) (gatewaydto.MessageHistoryPage, int, error) {
	if err := s.ensureLoaded(); err != nil {
		return gatewaydto.MessageHistoryPage{}, 0, err
	}
	conversation, err := s.conversations.Get(ctx, conversationID)
	if err != nil {
		return gatewaydto.MessageHistoryPage{}, 0, err
	}
	appID := conversation.AppKey

	before := len(s.List(ctx, conversationID, 0, 0, 0))
	if len(req.Items) > 0 {
		if _, err := s.CreateChannelMessages(ctx, appID, req); err != nil {
			return gatewaydto.MessageHistoryPage{}, 0, err
		}
	}
	after := len(s.List(ctx, conversationID, 0, 0, 0))
	page, err := s.History(ctx, appID, conversationID, filter)
	if err != nil {
		return gatewaydto.MessageHistoryPage{}, 0, err
	}
	createdCount := after - before
	if createdCount < 0 {
		createdCount = 0
	}
	return page, createdCount, nil
}

func (s *MessageService) Offset(_ context.Context, appID, conversationID string) (gatewaydto.MessageOffset, error) {
	if err := s.ensureLoaded(); err != nil {
		return gatewaydto.MessageOffset{}, err
	}
	conversation, err := s.conversations.Get(context.Background(), conversationID)
	if err != nil {
		return gatewaydto.MessageOffset{}, err
	}
	if conversation.AppKey != appID {
		return gatewaydto.MessageOffset{}, notFound("conversation %s not found", conversationID)
	}
	latest := s.List(context.Background(), conversationID, 0, 0, 1)
	if len(latest) == 0 {
		return gatewaydto.MessageOffset{ConversationKey: conversationID}, nil
	}
	message := latest[len(latest)-1]
	return gatewaydto.MessageOffset{
		ConversationKey:  conversationID,
		LatestSeq:        message.Seq,
		LatestMessageID:  message.MessageKey,
		LatestExternalID: message.SourceExternalID,
		LatestSentAt:     message.SourceSentAt,
	}, nil
}

func (s *MessageService) CreateChannelMessages(_ context.Context, appID string, req gatewaydto.BatchCreateMessagesRequest) (gatewaydto.BatchCreateMessagesResult, error) {
	if err := s.ensureLoaded(); err != nil {
		return gatewaydto.BatchCreateMessagesResult{}, err
	}
	if len(req.Items) == 0 {
		return gatewaydto.BatchCreateMessagesResult{}, nil
	}
	items := make([]gatewaydto.Message, 0, len(req.Items))
	for _, item := range req.Items {
		message, err := s.CreateChannelMessage(context.Background(), appID, item)
		if err != nil {
			return gatewaydto.BatchCreateMessagesResult{}, err
		}
		items = append(items, message)
	}
	return gatewaydto.BatchCreateMessagesResult{Items: items}, nil
}

func (s *MessageService) Load() error {
	return s.ensureLoaded()
}

func (s *MessageService) ensureLoaded() error {
	s.loadOnce.Do(func() {
		s.loadErr = s.load()
	})
	return s.loadErr
}

func (s *MessageService) load() error {
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
