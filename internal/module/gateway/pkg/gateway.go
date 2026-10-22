package gateway

import (
	"cs-im-server/internal/module/gateway/internal/model"
	gatewayruntime "cs-im-server/internal/module/gateway/internal/runtime"
)

type (
	RealtimeEvent = gatewayruntime.RealtimeEvent
	Message       = model.Message
)

type (
	AppAuthorizer       = gatewayruntime.AppAuthorizer
	ConversationIngress = gatewayruntime.ConversationIngress
	MessageIngress      = gatewayruntime.MessageIngress
	RealtimeSubscriber  = gatewayruntime.RealtimeSubscriber
	RealtimePublisher   = gatewayruntime.RealtimePublisher
	MessageEgress       = gatewayruntime.MessageEgress
)

func GetAppAuthorizer() AppAuthorizer {
	return gatewayruntime.AppAuthorizerCapability()
}

func GetConversationIngress() ConversationIngress {
	return gatewayruntime.ConversationIngressCapability()
}

func GetMessageIngress() MessageIngress {
	return gatewayruntime.MessageIngressCapability()
}

func GetMessageEgress() MessageEgress {
	return gatewayruntime.MessageEgressCapability()
}

func GetRealtimeSubscriber() RealtimeSubscriber {
	return gatewayruntime.RealtimeSubscriberCapability()
}

func GetRealtimePublisher() RealtimePublisher {
	return gatewayruntime.RealtimePublisherCapability()
}

func SetCustomerMessageObserver(fn func(Message)) {
	gatewayruntime.SetCustomerMessageObserver(fn)
}
