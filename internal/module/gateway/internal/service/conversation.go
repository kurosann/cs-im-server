package service

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	relaypkg "cs-im-server/internal/module/relay/pkg"
)

type ConversationService struct {
	mu                   sync.RWMutex
	conversationAccessor dataaccess.DataAccessor[*Conversation]
	deliveryAccessor     dataaccess.DataAccessor[*Delivery]
	conversations        map[string]*Conversation
	deliveries           map[string]*Delivery
	apps                 *AppService
	customers            *CustomerService
	messages             MessageStore
	realtime             *RealtimeService
}

type ConversationFilter struct {
	Keyword  string
	AppID    string
	Status   string
	Page     int
	PageSize int
}

type ConversationSummary struct {
	ConversationID string `json:"conversationId"`
	AppID          string `json:"appId"`
	CustomerID     string `json:"customerId"`
	CustomerName   string `json:"customerName"`
	Channel        string `json:"channel"`
	Subject        string `json:"subject"`
	Status         string `json:"status"`
	Assignee       string `json:"assignee"`
	LastMessage    string `json:"lastMessage"`
	UnreadCount    int    `json:"unreadCount"`
	ReplyStatus    string `json:"replyStatus"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type ConversationPage struct {
	Items []ConversationSummary `json:"items"`
	Total int64                 `json:"total"`
}

func NewConversationService(apps *AppService, customers *CustomerService, messages MessageStore, realtime *RealtimeService) *ConversationService {
	return &ConversationService{
		conversations: make(map[string]*Conversation),
		deliveries:    make(map[string]*Delivery),
		apps:          apps, customers: customers, messages: messages, realtime: realtime,
	}
}

func (s *ConversationService) SetMessages(messages MessageStore) {
	s.messages = messages
}

func (s *ConversationService) SetForTest(conversation Conversation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value := conversation
	s.conversations[conversation.ConversationKey] = &value
}

func (s *ConversationService) Ensure(ctx context.Context, appID string, req gatewaydto.EnsureConversationRequest) (Conversation, bool, error) {
	if appID == "" {
		return Conversation{}, false, badRequest("appId is required")
	}
	if req.CustomerExternalID == "" {
		return Conversation{}, false, badRequest("customerExternalId is required")
	}
	customer, err := s.customers.EnsureExternal(ctx, appID, gatewaydto.EnsureExternalCustomerRequest{
		ExternalID:  req.CustomerExternalID,
		DisplayName: req.CustomerName,
		Channel:     req.Channel,
	})
	if err != nil {
		return Conversation{}, false, err
	}

	s.mu.Lock()
	for _, conversation := range s.conversations {
		if conversation.AppKey == appID && conversation.CustomerKey == customer.CustomerKey && conversation.Status == "open" {
			result := *conversation
			s.mu.Unlock()
			return result, false, nil
		}
	}
	s.mu.Unlock()

	conversation, err := s.Create(ctx, gatewaydto.CreateConversationRequest{
		AppID:      appID,
		CustomerID: customer.CustomerKey,
		Channel:    req.Channel,
		Subject:    req.Subject,
	})
	if err != nil {
		return Conversation{}, false, err
	}
	return conversation, true, nil
}

func (s *ConversationService) Create(_ context.Context, req gatewaydto.CreateConversationRequest) (Conversation, error) {
	if req.AppID == "" {
		return Conversation{}, badRequest("appId is required")
	}
	if req.CustomerID == "" {
		return Conversation{}, badRequest("customerId is required")
	}
	app, err := s.apps.Get(context.Background(), req.AppID)
	if err != nil {
		return Conversation{}, err
	}
	if app.Status != "active" {
		return Conversation{}, badRequest("app %s is disabled", req.AppID)
	}
	if _, err := s.customers.Get(context.Background(), req.CustomerID); err != nil {
		return Conversation{}, err
	}
	conversation := &Conversation{ConversationKey: "conv_" + idutil.GenShortId(), AppKey: req.AppID, CustomerKey: req.CustomerID, Channel: req.Channel, Subject: req.Subject, Status: "open"}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.conversationAccessor.Save(context.Background(), conversation); err != nil {
		return Conversation{}, err
	}
	s.conversations[conversation.ConversationKey] = conversation
	result := *conversation
	s.realtime.Publish("conversation.created", conversation.ConversationKey, result)
	return result, nil
}

func (s *ConversationService) List(_ context.Context, appID, customerID, status string) []Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conversations := make([]Conversation, 0, len(s.conversations))
	for _, conversation := range s.conversations {
		if appID != "" && conversation.AppKey != appID {
			continue
		}
		if customerID != "" && conversation.CustomerKey != customerID {
			continue
		}
		if status != "" && conversation.Status != status {
			continue
		}
		conversations = append(conversations, *conversation)
	}
	sort.Slice(conversations, func(i, j int) bool { return conversations[i].CreateAt.Before(conversations[j].CreateAt) })
	return conversations
}

func (s *ConversationService) ListForAdmin(ctx context.Context, principal Principal, filter ConversationFilter) (ConversationPage, error) {
	allowedApps := make(map[string]struct{})
	appNames := make(map[string]string)
	for _, app := range s.apps.List(ctx, principal) {
		allowedApps[app.AppKey] = struct{}{}
		appNames[app.AppKey] = app.DisplayName
	}
	if filter.AppID != "" {
		if _, ok := allowedApps[filter.AppID]; !ok {
			return ConversationPage{Items: []ConversationSummary{}}, nil
		}
	}

	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	items := make([]Conversation, 0, len(s.conversations))
	s.mu.RLock()
	for _, conversation := range s.conversations {
		if _, ok := allowedApps[conversation.AppKey]; !ok {
			continue
		}
		if filter.AppID != "" && conversation.AppKey != filter.AppID {
			continue
		}
		if filter.Status != "" && filter.Status != "all" && conversation.Status != filter.Status {
			continue
		}
		if keyword != "" {
			searchText := strings.ToLower(strings.Join([]string{
				conversation.ConversationKey,
				conversation.CustomerKey,
				conversation.Subject,
				appNames[conversation.AppKey],
			}, " "))
			if !strings.Contains(searchText, keyword) {
				continue
			}
		}
		items = append(items, *conversation)
	}
	s.mu.RUnlock()

	lastActivity := make(map[string]time.Time)
	if s.messages != nil {
		for _, conversation := range items {
			if activity, err := s.messages.LastReceivedActivity(ctx, conversation.ConversationKey); err == nil {
				lastActivity[conversation.ConversationKey] = activity
			}
		}
	}
	sort.Slice(items, func(i, j int) bool {
		left := lastActivity[items[i].ConversationKey]
		if left.IsZero() {
			left = items[i].UpdateAt
		}
		right := lastActivity[items[j].ConversationKey]
		if right.IsZero() {
			right = items[j].UpdateAt
		}
		return left.After(right)
	})
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return ConversationPage{Items: []ConversationSummary{}}, nil
	}
	end := min(start+pageSize, len(items))

	result := ConversationPage{Items: make([]ConversationSummary, 0, end-start), Total: int64(len(items))}
	for _, conversation := range items[start:end] {
		summary := ConversationSummary{
			ConversationID: conversation.ConversationKey,
			AppID:          conversation.AppKey,
			CustomerID:     conversation.CustomerKey,
			Channel:        appNames[conversation.AppKey],
			Subject:        conversation.Subject,
			Status:         conversation.Status,
			Assignee:       conversation.Assignee,
			CreatedAt:      conversation.CreateAt.Format(time.RFC3339Nano),
			UpdatedAt:      conversation.UpdateAt.Format(time.RFC3339Nano),
		}
		if customer, err := s.customers.Get(context.Background(), conversation.CustomerKey); err == nil {
			summary.CustomerName = customer.Name
		}
		if activity, ok := lastActivity[conversation.ConversationKey]; ok {
			summary.UpdatedAt = activity.Format(time.RFC3339Nano)
		}
		if s.messages != nil {
			if message, err := s.messages.Latest(context.Background(), conversation.ConversationKey); err == nil {
				summary.LastMessage = message.Text
			}
			if stats, err := s.messages.ConversationStats(context.Background(), conversation.ConversationKey); err == nil {
				summary.UnreadCount = stats.UnreadCount
				summary.ReplyStatus = stats.ReplyStatus
			}
		}
		result.Items = append(result.Items, summary)
	}
	return result, nil
}

func (s *ConversationService) Get(_ context.Context, id string) (Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conversation, ok := s.conversations[id]
	if !ok {
		return Conversation{}, notFound("conversation %s not found", id)
	}
	return *conversation, nil
}

func (s *ConversationService) AdminEventAllowed(principal Principal, event RealtimeEvent) bool {
	if event.Type != "conversation.created" {
		return false
	}
	conversation, ok := event.Data.(Conversation)
	if !ok || principal.TenantID == "" {
		return false
	}
	app, err := s.apps.Get(context.Background(), conversation.AppKey)
	return err == nil && app.TenantID == principal.TenantID
}

func (s *ConversationService) MarkRead(ctx context.Context, id string) (int, error) {
	if _, err := s.Get(ctx, id); err != nil {
		return 0, err
	}
	if s.messages == nil {
		return 0, nil
	}
	return s.messages.MarkConversationRead(ctx, id)
}

func (s *ConversationService) Assign(_ context.Context, id string, req gatewaydto.AssignConversationRequest) (Conversation, error) {
	if req.Assignee == "" {
		return Conversation{}, badRequest("assignee is required")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	conversation, ok := s.conversations[id]
	if !ok {
		return Conversation{}, notFound("conversation %s not found", id)
	}
	if conversation.Status != "open" {
		return Conversation{}, badRequest("conversation %s is not open", id)
	}
	conversation.Assignee = req.Assignee
	conversation.UpdateAt = now
	if err := s.conversationAccessor.Save(context.Background(), conversation); err != nil {
		return Conversation{}, err
	}
	result := *conversation
	s.realtime.Publish("conversation.assigned", id, result)
	return result, nil
}

func (s *ConversationService) Close(_ context.Context, id string, req gatewaydto.CloseConversationRequest) (Conversation, error) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	conversation, ok := s.conversations[id]
	if !ok {
		return Conversation{}, notFound("conversation %s not found", id)
	}
	if conversation.Status == "closed" {
		return *conversation, nil
	}
	conversation.Status = "closed"
	conversation.UpdateAt = now
	conversation.ClosedAt = &now
	if req.Reason != "" {
		conversation.CloseReason = req.Reason
	}
	if err := s.conversationAccessor.Save(context.Background(), conversation); err != nil {
		return Conversation{}, err
	}
	result := *conversation
	s.realtime.Publish("conversation.closed", id, result)
	return result, nil
}

func (s *ConversationService) Forward(_ context.Context, id string, req gatewaydto.ForwardConversationRequest) (Delivery, error) {
	if req.ChannelID == "" {
		return Delivery{}, badRequest("channelId is required")
	}
	channel, err := relaypkg.Service().GetChannel(context.Background(), req.ChannelID)
	if err != nil {
		return Delivery{}, err
	}
	if channel.Status != "active" {
		return Delivery{}, badRequest("channel %s is disabled", req.ChannelID)
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	conversation, ok := s.conversations[id]
	if !ok {
		return Delivery{}, notFound("conversation %s not found", id)
	}
	messageID := req.MessageID
	if messageID == "" {
		latest, err := s.messages.Latest(context.Background(), id)
		if err != nil {
			return Delivery{}, err
		}
		messageID = latest.MessageKey
	} else if _, err := s.messages.Get(context.Background(), messageID); err != nil {
		return Delivery{}, err
	}
	delivery := &Delivery{DeliveryKey: "dlv_" + idutil.GenShortId(), ConversationKey: id, MessageKey: messageID, ChannelKey: channel.ChannelKey, Status: "queued"}
	if err := s.deliveryAccessor.Save(context.Background(), delivery); err != nil {
		return Delivery{}, err
	}
	s.deliveries[delivery.DeliveryKey] = delivery
	conversation.ForwardedTo = channel.ChannelKey
	conversation.UpdateAt = now
	if err := s.conversationAccessor.Save(context.Background(), conversation); err != nil {
		return Delivery{}, err
	}
	result := *delivery
	s.realtime.Publish("conversation.forwarded", id, result)
	return result, nil
}

func (s *ConversationService) Deliveries(_ context.Context, conversationID string) []Delivery {
	s.mu.RLock()
	defer s.mu.RUnlock()
	deliveries := make([]Delivery, 0, len(s.deliveries))
	for _, delivery := range s.deliveries {
		if delivery.ConversationKey != conversationID {
			continue
		}
		deliveries = append(deliveries, *delivery)
	}
	sort.Slice(deliveries, func(i, j int) bool { return deliveries[i].CreateAt.Before(deliveries[j].CreateAt) })
	return deliveries
}

func (s *ConversationService) RetargetCustomer(sourceID, targetID string) error {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, conversation := range s.conversations {
		if conversation.CustomerKey != sourceID {
			continue
		}
		conversation.CustomerKey = targetID
		conversation.UpdateAt = now
		if err := s.conversationAccessor.Save(context.Background(), conversation); err != nil {
			return err
		}
	}
	return nil
}

func (s *ConversationService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	conversations, err := s.conversationAccessor.List(context.Background())
	if err != nil {
		return err
	}
	for _, conversation := range conversations {
		s.conversations[conversation.ConversationKey] = conversation
	}
	deliveries, err := s.deliveryAccessor.List(context.Background())
	if err != nil {
		return err
	}
	for _, delivery := range deliveries {
		s.deliveries[delivery.DeliveryKey] = delivery
	}
	return nil
}
