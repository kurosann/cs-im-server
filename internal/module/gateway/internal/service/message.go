package service

import (
	"context"
	"time"

	"vane-common/pkg/idutil"
)

var messageStatuses = map[string]bool{
	"received":  true,
	"sent":      true,
	"delivered": true,
	"read":      true,
	"failed":    true,
}

func (s *Service) CreateMessage(_ context.Context, req CreateMessageRequest) (Message, error) {
	if req.ConversationID == "" {
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
	now := time.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	conversation, ok := s.conversations[req.ConversationID]
	if !ok {
		return Message{}, notFound("conversation %s not found", req.ConversationID)
	}
	if req.CustomerID == "" {
		req.CustomerID = conversation.CustomerID
	}
	if req.CustomerID != conversation.CustomerID {
		return Message{}, badRequest("customer %s does not belong to conversation", req.CustomerID)
	}
	if conversation.Status != "open" {
		return Message{}, badRequest("conversation %s is closed", conversation.ID)
	}
	message := &Message{
		ID:             "msg_" + idutil.GenShortId(),
		ConversationID: conversation.ID,
		CustomerID:     conversation.CustomerID,
		Sender:         req.Sender,
		Source:         req.Source,
		Type:           req.Type,
		Text:           req.Text,
		Status:         "received",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.messages[message.ID] = message
	result := *message
	s.publishLocked("message.created", message.ID, result)
	return result, nil
}

func (s *Service) UpdateMessageStatus(_ context.Context, id string, req UpdateMessageStatusRequest) (Message, error) {
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
	message.UpdatedAt = now
	result := *message
	s.publishLocked("message.status_changed", id, result)
	return result, nil
}
