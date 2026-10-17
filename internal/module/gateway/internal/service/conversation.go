package service

import (
	"context"
	"sort"
	"time"

	"vane-common/pkg/idutil"
)

func (s *Service) CreateConversation(_ context.Context, req CreateConversationRequest) (Conversation, error) {
	if req.AppID == "" {
		return Conversation{}, badRequest("appId is required")
	}
	if req.CustomerID == "" {
		return Conversation{}, badRequest("customerId is required")
	}
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	app, ok := s.apps[req.AppID]
	if !ok {
		return Conversation{}, notFound("app %s not found", req.AppID)
	}
	if app.Status != "active" {
		return Conversation{}, badRequest("app %s is disabled", req.AppID)
	}
	if _, ok := s.customers[req.CustomerID]; !ok {
		return Conversation{}, notFound("customer %s not found", req.CustomerID)
	}
	conversation := &Conversation{
		ID:         "conv_" + idutil.GenShortId(),
		AppID:      req.AppID,
		CustomerID: req.CustomerID,
		Channel:    req.Channel,
		Subject:    req.Subject,
		Status:     "open",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.conversations[conversation.ID] = conversation
	result := *conversation
	s.publishLocked("conversation.created", conversation.ID, result)
	return result, nil
}

func (s *Service) ListConversations(_ context.Context, appID, customerID, status string) []Conversation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conversations := make([]Conversation, 0, len(s.conversations))
	for _, conversation := range s.conversations {
		if appID != "" && conversation.AppID != appID {
			continue
		}
		if customerID != "" && conversation.CustomerID != customerID {
			continue
		}
		if status != "" && conversation.Status != status {
			continue
		}
		conversations = append(conversations, *conversation)
	}
	sort.Slice(conversations, func(i, j int) bool {
		return conversations[i].CreatedAt.Before(conversations[j].CreatedAt)
	})
	return conversations
}

func (s *Service) GetConversation(_ context.Context, id string) (Conversation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conversation, ok := s.conversations[id]
	if !ok {
		return Conversation{}, notFound("conversation %s not found", id)
	}
	return *conversation, nil
}

func (s *Service) AssignConversation(_ context.Context, id string, req AssignConversationRequest) (Conversation, error) {
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
	conversation.UpdatedAt = now
	result := *conversation
	s.publishLocked("conversation.assigned", id, result)
	return result, nil
}

func (s *Service) CloseConversation(_ context.Context, id string, req CloseConversationRequest) (Conversation, error) {
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
	conversation.UpdatedAt = now
	conversation.ClosedAt = &now
	if req.Reason != "" {
		conversation.Subject = req.Reason
	}
	result := *conversation
	s.publishLocked("conversation.closed", id, result)
	return result, nil
}

func (s *Service) ForwardConversation(_ context.Context, id string, req ForwardConversationRequest) (Delivery, error) {
	if req.ChannelID == "" {
		return Delivery{}, badRequest("channelId is required")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	conversation, ok := s.conversations[id]
	if !ok {
		return Delivery{}, notFound("conversation %s not found", id)
	}
	channel, ok := s.channels[req.ChannelID]
	if !ok {
		return Delivery{}, notFound("channel %s not found", req.ChannelID)
	}
	if channel.Status != "active" {
		return Delivery{}, badRequest("channel %s is disabled", req.ChannelID)
	}
	messageID := req.MessageID
	if messageID == "" {
		for _, message := range s.messages {
			if message.ConversationID == id {
				messageID = message.ID
			}
		}
	}
	if messageID == "" {
		return Delivery{}, badRequest("conversation has no message to forward")
	}
	delivery := &Delivery{
		ID:             "dlv_" + idutil.GenShortId(),
		ConversationID: id,
		MessageID:      messageID,
		ChannelID:      channel.ID,
		Status:         "queued",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.deliveries[delivery.ID] = delivery
	conversation.ForwardedTo = channel.ID
	conversation.UpdatedAt = now
	result := *delivery
	s.publishLocked("conversation.forwarded", id, result)
	return result, nil
}

func (s *Service) ConversationMessages(_ context.Context, conversationID string) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	messages := make([]Message, 0, len(s.messages))
	for _, message := range s.messages {
		if message.ConversationID != conversationID {
			continue
		}
		messages = append(messages, *message)
	}
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})
	return messages
}

func (s *Service) ConversationDeliveries(_ context.Context, conversationID string) []Delivery {
	s.mu.RLock()
	defer s.mu.RUnlock()
	deliveries := make([]Delivery, 0, len(s.deliveries))
	for _, delivery := range s.deliveries {
		if delivery.ConversationID != conversationID {
			continue
		}
		deliveries = append(deliveries, *delivery)
	}
	sort.Slice(deliveries, func(i, j int) bool {
		return deliveries[i].CreatedAt.Before(deliveries[j].CreatedAt)
	})
	return deliveries
}
