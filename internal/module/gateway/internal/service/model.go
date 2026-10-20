package service

import (
	"context"
	"time"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"cs-im-server/internal/module/gateway/internal/model"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	relaypkg "cs-im-server/internal/module/relay/pkg"
)

type (
	App              = model.App
	Conversation     = model.Conversation
	Customer         = model.Customer
	CustomerIdentity = model.CustomerIdentity
	Message          = model.Message
	MessageSender    = model.MessageSender
	MessageSource    = model.MessageSource
	MessageMedia     = model.MessageMedia
	Principal        = iampkg.Principal
	RelayService     = relaypkg.RelayService
	Delivery         = relaypkg.Delivery
)

type ConversationRetargeter interface {
	RetargetCustomer(sourceID, targetID string) error
}

type MessageStore interface {
	Get(ctx context.Context, id string) (gatewaydto.MessageRef, error)
	Latest(ctx context.Context, conversationID string) (Message, error)
	ConversationStats(ctx context.Context, conversationID string) (ConversationMessageStats, error)
	LastReceivedActivity(ctx context.Context, conversationID string) (time.Time, error)
	MarkConversationRead(ctx context.Context, conversationID string) (int, error)
}

type ConversationMessageStats struct {
	UnreadCount int
	ReplyStatus string
}
