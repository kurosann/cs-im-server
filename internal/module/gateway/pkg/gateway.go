package gateway

import (
	"cs-im-server/internal/module/gateway/model"
	"cs-im-server/internal/module/gateway/service"
	"sync"
)

type (
	App                = model.App
	AppService         = service.AppService
	RegisterAppRequest = service.RegisterAppRequest
	HeartbeatRequest   = service.HeartbeatRequest

	Customer         = model.Customer
	CustomerIdentity = model.CustomerIdentity
	CustomerService  = service.CustomerService

	Conversation               = model.Conversation
	ConversationService        = service.ConversationService
	CreateConversationRequest  = service.CreateConversationRequest
	AssignConversationRequest  = service.AssignConversationRequest
	CloseConversationRequest   = service.CloseConversationRequest
	ForwardConversationRequest = service.ForwardConversationRequest
	EnsureConversationRequest  = service.EnsureConversationRequest

	Message                    = model.Message
	MessageSender              = model.MessageSender
	MessageSource              = model.MessageSource
	MessageService             = service.MessageService
	CreateMessageRequest       = service.CreateMessageRequest
	UpdateMessageStatusRequest = service.UpdateMessageStatusRequest
	MessageHistoryFilter       = service.MessageHistoryFilter
	MessageHistoryPage         = service.MessageHistoryPage

	ConversationMessageStore = service.MessageStore

	RealtimeEvent   = service.RealtimeEvent
	RealtimeService = service.RealtimeService
)

var (
	serviceMu        sync.RWMutex
	realtimeService  *RealtimeService
	appService       *AppService
	customerService  *CustomerService
	conversationServ *ConversationService
	messageService   *MessageService
)

func SetServices(
	realtime *RealtimeService, apps *AppService, customers *CustomerService,
	conversations *ConversationService, messages *MessageService,
) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	realtimeService = realtime
	appService = apps
	customerService = customers
	conversationServ = conversations
	messageService = messages
}

func GetRealtime() *RealtimeService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if realtimeService == nil {
		panic("gateway realtime service is not initialized")
	}
	return realtimeService
}

func GetApp() *AppService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if appService == nil {
		panic("gateway app service is not initialized")
	}
	return appService
}

func GetCustomer() *CustomerService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if customerService == nil {
		panic("gateway customer service is not initialized")
	}
	return customerService
}

func GetConversation() *ConversationService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if conversationServ == nil {
		panic("gateway conversation service is not initialized")
	}
	return conversationServ
}

func GetMessage() *MessageService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if messageService == nil {
		panic("gateway message service is not initialized")
	}
	return messageService
}

func NewRealtimeService() *RealtimeService { return service.NewRealtimeService() }

func NewAppService(realtime *RealtimeService) *AppService {
	return service.NewAppService(realtime)
}

func NewCustomerService(realtime *RealtimeService) *CustomerService {
	return service.NewCustomerService(realtime)
}

func NewConversationService(
	apps *AppService, customers *CustomerService, messages ConversationMessageStore, realtime *RealtimeService,
) *ConversationService {
	return service.NewConversationService(apps, customers, messages, realtime)
}

func NewMessageService(
	apps *AppService, conversations *ConversationService, realtime *RealtimeService,
) *MessageService {
	return service.NewMessageService(apps, conversations, realtime)
}

func NewConversationMessageStore(messages *MessageService) ConversationMessageStore {
	return service.NewConversationMessageStore(messages)
}
