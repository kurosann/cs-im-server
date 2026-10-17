package service

import (
	"context"

	conversationmodel "cs-im-server/internal/module/gateway/internal/conversation/model"

	apppkg "cs-im-server/internal/module/gateway/internal/app/pkg"
	customerpkg "cs-im-server/internal/module/gateway/internal/customer/pkg"
	realtimepkg "cs-im-server/internal/module/gateway/internal/realtime/pkg"
	relaypkg "cs-im-server/internal/module/relay/pkg"
)

type Conversation = conversationmodel.Conversation

type AppService = apppkg.AppService
type CustomerService = customerpkg.CustomerService
type RelayService = relaypkg.RelayService
type Delivery = relaypkg.Delivery
type RealtimeService = realtimepkg.RealtimeService
type EnsureExternalCustomerRequest = customerpkg.EnsureExternalCustomerRequest

type MessageRef struct {
	MessageKey string
}

type MessageStore interface {
	Get(ctx context.Context, id string) (MessageRef, error)
	Latest(ctx context.Context, conversationID string) (MessageRef, error)
}
