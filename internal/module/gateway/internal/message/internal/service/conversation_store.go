package service

import (
	"context"

	conversationpkg "cs-im-server/internal/module/gateway/internal/conversation/pkg"
)

type ConversationMessageStore struct {
	messages *MessageService
}

func NewConversationMessageStore(messages *MessageService) *ConversationMessageStore {
	return &ConversationMessageStore{messages: messages}
}

func (s *ConversationMessageStore) Get(ctx context.Context, id string) (conversationpkg.MessageRef, error) {
	message, err := s.messages.Get(ctx, id)
	if err != nil {
		return conversationpkg.MessageRef{}, err
	}
	return conversationpkg.MessageRef{MessageKey: message.MessageKey}, nil
}

func (s *ConversationMessageStore) Latest(ctx context.Context, conversationID string) (conversationpkg.MessageRef, error) {
	message, err := s.messages.Latest(ctx, conversationID)
	if err != nil {
		return conversationpkg.MessageRef{}, err
	}
	return conversationpkg.MessageRef{MessageKey: message.MessageKey}, nil
}

var _ conversationpkg.MessageStore = (*ConversationMessageStore)(nil)
