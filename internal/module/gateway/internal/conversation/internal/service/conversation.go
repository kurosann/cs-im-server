package service

import (
	"context"
	"sort"
	"sync"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

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

type CreateConversationRequest struct {
	AppID      string `json:"appId"`
	CustomerID string `json:"customerId"`
	Channel    string `json:"channel"`
	Subject    string `json:"subject"`
}

type AssignConversationRequest struct {
	Assignee string `json:"assignee"`
}

type CloseConversationRequest struct {
	Reason string `json:"reason"`
}

type ForwardConversationRequest struct {
	MessageID string `json:"messageId"`
	ChannelID string `json:"channelId"`
}

type EnsureConversationRequest struct {
	CustomerExternalID string `json:"customerExternalId"`
	CustomerName       string `json:"customerName"`
	Channel            string `json:"channel"`
	Subject            string `json:"subject"`
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

func (s *ConversationService) Ensure(ctx context.Context, appID string, req EnsureConversationRequest) (Conversation, bool, error) {
	if appID == "" {
		return Conversation{}, false, badRequest("appId is required")
	}
	if req.CustomerExternalID == "" {
		return Conversation{}, false, badRequest("customerExternalId is required")
	}
	customer, err := s.customers.EnsureExternal(ctx, appID, EnsureExternalCustomerRequest{
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

	conversation, err := s.Create(ctx, CreateConversationRequest{
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

func (s *ConversationService) Create(_ context.Context, req CreateConversationRequest) (Conversation, error) {
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

func (s *ConversationService) Get(_ context.Context, id string) (Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conversation, ok := s.conversations[id]
	if !ok {
		return Conversation{}, notFound("conversation %s not found", id)
	}
	return *conversation, nil
}

func (s *ConversationService) Assign(_ context.Context, id string, req AssignConversationRequest) (Conversation, error) {
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

func (s *ConversationService) Close(_ context.Context, id string, req CloseConversationRequest) (Conversation, error) {
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

func (s *ConversationService) Forward(_ context.Context, id string, req ForwardConversationRequest) (Delivery, error) {
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
