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
	conversationOnce     sync.Once
	conversationErr      error
	deliveryOnce         sync.Once
	deliveryErr          error
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
	LastMessageAt  string `json:"lastMessageAt,omitempty"`
	UnreadCount    int    `json:"unreadCount"`
	ReplyStatus    string `json:"replyStatus"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
}

type ConversationPage struct {
	Items []ConversationSummary `json:"items"`
	Total int64                 `json:"total"`
}

type DeliveryStats struct {
	Total       int        `json:"total"`
	Queued      int        `json:"queued"`
	Delivered   int        `json:"delivered"`
	Failed      int        `json:"failed"`
	LastEventAt *time.Time `json:"lastEventAt,omitempty"`
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
	s.conversationOnce.Do(func() {})
	s.mu.Lock()
	defer s.mu.Unlock()
	value := conversation
	s.conversations[conversation.ConversationKey] = &value
}

func (s *ConversationService) Ensure(ctx context.Context, appID string, req gatewaydto.EnsureConversationRequest) (gatewaydto.Conversation, bool, error) {
	if err := s.ensureConversations(); err != nil {
		return gatewaydto.Conversation{}, false, err
	}
	if appID == "" {
		return gatewaydto.Conversation{}, false, badRequest("appId is required")
	}
	if req.CustomerExternalID == "" {
		return gatewaydto.Conversation{}, false, badRequest("customerExternalId is required")
	}
	customer, err := s.customers.EnsureExternal(ctx, appID, gatewaydto.EnsureExternalCustomerRequest{
		ExternalID:  req.CustomerExternalID,
		DisplayName: req.CustomerName,
		Channel:     req.Channel,
	})
	if err != nil {
		return gatewaydto.Conversation{}, false, err
	}

	s.mu.Lock()
	for _, conversation := range s.conversations {
		if conversation.AppKey == appID && conversation.CustomerKey == customer.CustomerKey && conversation.Status == "open" {
			s.mu.Unlock()
			return conversationToDTO(conversation), false, nil
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
		return gatewaydto.Conversation{}, false, err
	}
	return conversation, true, nil
}

func (s *ConversationService) Create(_ context.Context, req gatewaydto.CreateConversationRequest) (gatewaydto.Conversation, error) {
	if err := s.ensureConversations(); err != nil {
		return gatewaydto.Conversation{}, err
	}
	if req.AppID == "" {
		return gatewaydto.Conversation{}, badRequest("appId is required")
	}
	if req.CustomerID == "" {
		return gatewaydto.Conversation{}, badRequest("customerId is required")
	}
	app, err := s.apps.Get(context.Background(), req.AppID)
	if err != nil {
		return gatewaydto.Conversation{}, err
	}
	if app.Status != "active" {
		return gatewaydto.Conversation{}, badRequest("app %s is disabled", req.AppID)
	}
	if _, err := s.customers.Get(context.Background(), req.CustomerID); err != nil {
		return gatewaydto.Conversation{}, err
	}
	conversation := &Conversation{ConversationKey: "conv_" + idutil.GenShortId(), AppKey: req.AppID, CustomerKey: req.CustomerID, Channel: req.Channel, Subject: req.Subject, Status: "open"}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.conversationAccessor.Save(context.Background(), conversation); err != nil {
		return gatewaydto.Conversation{}, err
	}
	s.conversations[conversation.ConversationKey] = conversation
	dto := conversationToDTO(conversation)
	s.realtime.Publish("conversation.created", conversation.ConversationKey, dto)
	return dto, nil
}

func (s *ConversationService) List(_ context.Context, appID, customerID, status string) []Conversation {
	if err := s.ensureConversations(); err != nil {
		return []Conversation{}
	}
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
	if err := s.ensureConversations(); err != nil {
		return ConversationPage{}, err
	}
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
			summary.LastMessageAt = activity.Format(time.RFC3339Nano)
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

func (s *ConversationService) Get(_ context.Context, id string) (gatewaydto.Conversation, error) {
	if err := s.ensureConversations(); err != nil {
		return gatewaydto.Conversation{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	conversation, ok := s.conversations[id]
	if !ok {
		return gatewaydto.Conversation{}, notFound("conversation %s not found", id)
	}
	return conversationToDTO(conversation), nil
}

func (s *ConversationService) AdminEventAllowed(principal Principal, event RealtimeEvent) bool {
	if err := s.ensureConversations(); err != nil {
		return false
	}
	if event.Type != "conversation.created" {
		return false
	}
	conversation, ok := event.Data.(gatewaydto.Conversation)
	if !ok || principal.TenantID == "" {
		return false
	}
	app, err := s.apps.Get(context.Background(), conversation.AppKey)
	return err == nil && app.TenantID == principal.TenantID
}

func (s *ConversationService) MarkRead(ctx context.Context, id string) (int, error) {
	if err := s.ensureConversations(); err != nil {
		return 0, err
	}
	if _, err := s.Get(ctx, id); err != nil {
		return 0, err
	}
	if s.messages == nil {
		return 0, nil
	}
	return s.messages.MarkConversationRead(ctx, id)
}

func (s *ConversationService) Assign(_ context.Context, id string, req gatewaydto.AssignConversationRequest) (Conversation, error) {
	if err := s.ensureConversations(); err != nil {
		return Conversation{}, err
	}
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
	s.realtime.Publish("conversation.assigned", id, conversationToDTO(&result))
	return result, nil
}

func (s *ConversationService) Close(_ context.Context, id string, req gatewaydto.CloseConversationRequest) (Conversation, error) {
	if err := s.ensureConversations(); err != nil {
		return Conversation{}, err
	}
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
	s.realtime.Publish("conversation.closed", id, conversationToDTO(&result))
	return result, nil
}

func (s *ConversationService) Forward(_ context.Context, id string, req gatewaydto.ForwardConversationRequest) (Delivery, error) {
	if err := s.ensureConversations(); err != nil {
		return Delivery{}, err
	}
	if err := s.ensureDeliveries(); err != nil {
		return Delivery{}, err
	}
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
	s.realtime.Publish("conversation.forwarded", id, conversationToDTO(conversation))
	return result, nil
}

func (s *ConversationService) Deliveries(_ context.Context, conversationID string) []Delivery {
	if err := s.ensureDeliveries(); err != nil {
		return []Delivery{}
	}
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

func (s *ConversationService) DeliveryStatsByChannel() map[string]DeliveryStats {
	if err := s.ensureDeliveries(); err != nil {
		return map[string]DeliveryStats{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := make(map[string]DeliveryStats)
	for _, delivery := range s.deliveries {
		item := stats[delivery.ChannelKey]
		item.Total++
		switch delivery.Status {
		case "queued":
			item.Queued++
		case "delivered":
			item.Delivered++
		case "failed":
			item.Failed++
		}
		lastEventAt := delivery.UpdateAt
		if item.LastEventAt == nil || lastEventAt.After(*item.LastEventAt) {
			item.LastEventAt = &lastEventAt
		}
		stats[delivery.ChannelKey] = item
	}
	return stats
}

func (s *ConversationService) ConversationCountByApp() map[string]int {
	if err := s.ensureConversations(); err != nil {
		return map[string]int{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	counts := make(map[string]int)
	for _, conversation := range s.conversations {
		counts[conversation.AppKey]++
	}
	return counts
}

func (s *ConversationService) RetargetCustomer(sourceID, targetID string) error {
	if err := s.ensureConversations(); err != nil {
		return err
	}
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
	if err := s.ensureConversations(); err != nil {
		return err
	}
	return s.ensureDeliveries()
}

func (s *ConversationService) ensureConversations() error {
	s.conversationOnce.Do(func() {
		s.conversationErr = s.loadConversations()
	})
	return s.conversationErr
}

func (s *ConversationService) ensureDeliveries() error {
	s.deliveryOnce.Do(func() {
		s.deliveryErr = s.loadDeliveries()
	})
	return s.deliveryErr
}

func (s *ConversationService) loadConversations() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	conversations, err := s.conversationAccessor.List(context.Background())
	if err != nil {
		return err
	}
	for _, conversation := range conversations {
		s.conversations[conversation.ConversationKey] = conversation
	}
	return nil
}

func (s *ConversationService) loadDeliveries() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	deliveries, err := s.deliveryAccessor.List(context.Background())
	if err != nil {
		return err
	}
	for _, delivery := range deliveries {
		s.deliveries[delivery.DeliveryKey] = delivery
	}
	return nil
}
