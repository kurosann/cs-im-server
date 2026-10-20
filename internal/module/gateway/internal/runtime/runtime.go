package runtime

import (
	"context"
	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"sync"

	"cs-im-server/internal/module/gateway/internal/model"
	"cs-im-server/internal/module/gateway/internal/service"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
)

type AppAuthorizer interface {
	AuthorizeAccount(appID string, principal iampkg.Principal) error
}

type ConversationIngress interface {
	Ensure(ctx context.Context, appID string, req gatewaydto.EnsureConversationRequest) (model.Conversation, bool, error)
	Get(ctx context.Context, id string) (model.Conversation, error)
}

type MessageIngress interface {
	CreateChannelMessage(ctx context.Context, appID string, req gatewaydto.CreateMessageRequest) (model.Message, error)
	History(ctx context.Context, appID, conversationID string, filter gatewaydto.MessageHistoryFilter) (gatewaydto.MessageHistoryPage, error)
	ChannelEventAllowed(appID string, event service.RealtimeEvent) bool
}

type MessageEgress interface {
	CreateRelayMessage(ctx context.Context, relayID string, req gatewaydto.CreateMessageRequest) (model.Message, error)
	RelayHistory(ctx context.Context, conversationID string, filter gatewaydto.MessageHistoryFilter) (gatewaydto.MessageHistoryPage, error)
	RelayEventAllowed(event service.RealtimeEvent) bool
}

type RealtimeSubscriber interface {
	Subscribe(ctx context.Context) (<-chan service.RealtimeEvent, func(), error)
}

type RealtimePublisher interface {
	Publish(eventType, id string, data any)
}

var (
	serviceMu           sync.RWMutex
	appAuthorizer       AppAuthorizer
	conversationIngress ConversationIngress
	messageIngress      MessageIngress
	messageEgress       MessageEgress
	realtimeSubscriber  RealtimeSubscriber
	realtimePublisher   RealtimePublisher
)

func SetCapabilities(
	apps AppAuthorizer, conversations ConversationIngress, messages MessageIngress,
	subscriber RealtimeSubscriber, publisher RealtimePublisher,
	egress MessageEgress,
) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	appAuthorizer = apps
	conversationIngress = conversations
	messageIngress = messages
	messageEgress = egress
	realtimeSubscriber = subscriber
	realtimePublisher = publisher
}

func AppAuthorizerCapability() AppAuthorizer {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if appAuthorizer == nil {
		panic("gateway app authorizer is not initialized")
	}
	return appAuthorizer
}

func ConversationIngressCapability() ConversationIngress {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if conversationIngress == nil {
		panic("gateway conversation ingress is not initialized")
	}
	return conversationIngress
}

func MessageIngressCapability() MessageIngress {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if messageIngress == nil {
		panic("gateway message ingress is not initialized")
	}
	return messageIngress
}

func MessageEgressCapability() MessageEgress {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if messageEgress == nil {
		panic("gateway message egress is not initialized")
	}
	return messageEgress
}

func RealtimeSubscriberCapability() RealtimeSubscriber {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if realtimeSubscriber == nil {
		panic("gateway realtime subscriber is not initialized")
	}
	return realtimeSubscriber
}

func RealtimePublisherCapability() RealtimePublisher {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if realtimePublisher == nil {
		panic("gateway realtime publisher is not initialized")
	}
	return realtimePublisher
}
