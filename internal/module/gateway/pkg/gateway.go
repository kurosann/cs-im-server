package gateway

import (
	"context"
	"cs-im-server/internal/module/gateway/internal/model"
	"cs-im-server/internal/module/gateway/internal/service"
	iampkg "cs-im-server/internal/module/iam/pkg"
	"sync"
)

type (
	App                = model.App
	RegisterAppRequest = service.RegisterAppRequest
	HeartbeatRequest   = service.HeartbeatRequest

	Customer         = model.Customer
	CustomerIdentity = model.CustomerIdentity

	Conversation               = model.Conversation
	CreateConversationRequest  = service.CreateConversationRequest
	AssignConversationRequest  = service.AssignConversationRequest
	CloseConversationRequest   = service.CloseConversationRequest
	ForwardConversationRequest = service.ForwardConversationRequest
	EnsureConversationRequest  = service.EnsureConversationRequest

	Message                    = model.Message
	MessageSender              = model.MessageSender
	MessageSource              = model.MessageSource
	CreateMessageRequest       = service.CreateMessageRequest
	UpdateMessageStatusRequest = service.UpdateMessageStatusRequest
	MessageHistoryFilter       = service.MessageHistoryFilter
	MessageHistoryPage         = service.MessageHistoryPage

	RealtimeEvent = service.RealtimeEvent
)

type AppAuthorizer interface {
	AuthorizeAccount(appID string, principal iampkg.Principal) error
}

type ConversationIngress interface {
	Ensure(ctx context.Context, appID string, req EnsureConversationRequest) (Conversation, bool, error)
	Get(ctx context.Context, id string) (Conversation, error)
}

type MessageIngress interface {
	CreateChannelMessage(ctx context.Context, appID string, req CreateMessageRequest) (Message, error)
	History(ctx context.Context, appID, conversationID string, filter MessageHistoryFilter) (MessageHistoryPage, error)
	ChannelEventAllowed(appID string, event RealtimeEvent) bool
}

type RealtimeSubscriber interface {
	Subscribe(ctx context.Context) (<-chan RealtimeEvent, func(), error)
}

type RealtimePublisher interface {
	Publish(eventType, id string, data any)
}

var (
	serviceMu           sync.RWMutex
	appAuthorizer       AppAuthorizer
	conversationIngress ConversationIngress
	messageIngress      MessageIngress
	realtimeSubscriber  RealtimeSubscriber
	realtimePublisher   RealtimePublisher
)

func SetCapabilities(
	apps AppAuthorizer, conversations ConversationIngress, messages MessageIngress,
	subscriber RealtimeSubscriber, publisher RealtimePublisher,
) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	appAuthorizer = apps
	conversationIngress = conversations
	messageIngress = messages
	realtimeSubscriber = subscriber
	realtimePublisher = publisher
}

func GetAppAuthorizer() AppAuthorizer {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if appAuthorizer == nil {
		panic("gateway app authorizer is not initialized")
	}
	return appAuthorizer
}

func GetConversationIngress() ConversationIngress {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if conversationIngress == nil {
		panic("gateway conversation ingress is not initialized")
	}
	return conversationIngress
}

func GetMessageIngress() MessageIngress {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if messageIngress == nil {
		panic("gateway message ingress is not initialized")
	}
	return messageIngress
}

func GetRealtimeSubscriber() RealtimeSubscriber {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if realtimeSubscriber == nil {
		panic("gateway realtime subscriber is not initialized")
	}
	return realtimeSubscriber
}

func GetRealtimePublisher() RealtimePublisher {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if realtimePublisher == nil {
		panic("gateway realtime publisher is not initialized")
	}
	return realtimePublisher
}
