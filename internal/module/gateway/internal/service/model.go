package service

import (
	"context"

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
	Latest(ctx context.Context, conversationID string) (gatewaydto.MessageRef, error)
}
