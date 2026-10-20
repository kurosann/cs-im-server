package service

import (
	"context"
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

func (s *ConversationMessageStore) Latest(ctx context.Context, conversationID string) (gatewaydto.MessageRef, error) {
	message, err := s.messages.Latest(ctx, conversationID)
	if err != nil {
		return gatewaydto.MessageRef{}, err
	}
	return gatewaydto.MessageRef{MessageKey: message.MessageKey}, nil
}

var _ MessageStore = (*ConversationMessageStore)(nil)
