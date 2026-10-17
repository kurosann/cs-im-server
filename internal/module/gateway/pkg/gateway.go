package gateway

import (
	"cs-im-server/internal/module/gateway/internal/model"
	gatewayruntime "cs-im-server/internal/module/gateway/internal/runtime"
	"cs-im-server/internal/module/gateway/internal/service"
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
