package service

import (
	"context"

	"cs-im-server/internal/module/gateway/internal/model"
	iampkg "cs-im-server/internal/module/iam/pkg"
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

type MessageRef struct {
	MessageKey string
}

type MessageStore interface {
	Get(ctx context.Context, id string) (MessageRef, error)
	Latest(ctx context.Context, conversationID string) (MessageRef, error)
}
