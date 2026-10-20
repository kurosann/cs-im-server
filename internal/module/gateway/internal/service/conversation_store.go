package service

import (
	"context"
	"time"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
)

type ConversationMessageStore struct {
	messages *MessageService
}

func NewConversationMessageStore(messages *MessageService) *ConversationMessageStore {
	return &ConversationMessageStore{messages: messages}
}

func (s *ConversationMessageStore) Get(ctx context.Context, id string) (gatewaydto.MessageRef, error) {
	message, err := s.messages.Get(ctx, id)
	if err != nil {
		return gatewaydto.MessageRef{}, err
	}
	return gatewaydto.MessageRef{MessageKey: message.MessageKey}, nil
}

func (s *ConversationMessageStore) Latest(ctx context.Context, conversationID string) (Message, error) {
	return s.messages.Latest(ctx, conversationID)
}

func (s *ConversationMessageStore) ConversationStats(ctx context.Context, conversationID string) (ConversationMessageStats, error) {
	return s.messages.ConversationStats(ctx, conversationID)
}

func (s *ConversationMessageStore) LastReceivedActivity(ctx context.Context, conversationID string) (time.Time, error) {
	return s.messages.LastReceivedActivity(ctx, conversationID)
}

func (s *ConversationMessageStore) MarkConversationRead(ctx context.Context, conversationID string) (int, error) {
	return s.messages.MarkConversationRead(ctx, conversationID)
}

var _ MessageStore = (*ConversationMessageStore)(nil)
