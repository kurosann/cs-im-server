package gateway

import (
	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"cs-im-server/internal/module/gateway/internal/model"
	gatewayruntime "cs-im-server/internal/module/gateway/internal/runtime"
	"cs-im-server/internal/module/gateway/internal/service"
)

type (
	App                = model.App
	RegisterAppRequest = gatewaydto.RegisterAppRequest
	HeartbeatRequest   = gatewaydto.HeartbeatRequest

	Customer         = model.Customer
	CustomerIdentity = model.CustomerIdentity

	Conversation               = model.Conversation
	CreateConversationRequest  = gatewaydto.CreateConversationRequest
	AssignConversationRequest  = gatewaydto.AssignConversationRequest
	CloseConversationRequest   = gatewaydto.CloseConversationRequest
	ForwardConversationRequest = gatewaydto.ForwardConversationRequest
	EnsureConversationRequest  = gatewaydto.EnsureConversationRequest

	Message                    = model.Message
	MessageSender              = model.MessageSender
	MessageSource              = model.MessageSource
	MessageMedia               = model.MessageMedia
	CreateMessageRequest       = gatewaydto.CreateMessageRequest
	UpdateMessageStatusRequest = gatewaydto.UpdateMessageStatusRequest
	MessageHistoryFilter       = gatewaydto.MessageHistoryFilter
	MessageHistoryPage         = gatewaydto.MessageHistoryPage

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

func SetCustomerMessageObserver(fn func(Message)) {
	gatewayruntime.SetCustomerMessageObserver(fn)
}
