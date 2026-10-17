package conversation

import "sync"

import (
	commonmodule "vane-common/module"

	apppkg "cs-im-server/internal/module/gateway/internal/app/pkg"
	conversationhttp "cs-im-server/internal/module/gateway/internal/conversation/http"
	service "cs-im-server/internal/module/gateway/internal/conversation/internal/service"
	model "cs-im-server/internal/module/gateway/internal/conversation/model"
	customerpkg "cs-im-server/internal/module/gateway/internal/customer/pkg"
	realtimepkg "cs-im-server/internal/module/gateway/internal/realtime/pkg"
)

type (
	Conversation               = model.Conversation
	ConversationService        = service.ConversationService
	CreateConversationRequest  = service.CreateConversationRequest
	AssignConversationRequest  = service.AssignConversationRequest
	CloseConversationRequest   = service.CloseConversationRequest
	ForwardConversationRequest = service.ForwardConversationRequest
	EnsureConversationRequest  = service.EnsureConversationRequest
)

var serviceMu sync.RWMutex
var conversationService *ConversationService

func SetService(next *ConversationService) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	conversationService = next
}

func Service() *ConversationService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if conversationService == nil {
		panic("conversation service is not initialized")
	}
	return conversationService
}

func SetMessages(messages MessageStore) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	if conversationService != nil {
		conversationService.SetMessages(messages)
	}
}

type MessageRef = service.MessageRef

type MessageStore = service.MessageStore

func Init(apps *apppkg.AppService, customers *customerpkg.CustomerService, realtime *realtimepkg.RealtimeService) error {
	next := service.NewConversationService(apps, customers, nil, realtime)
	if err := next.Load(); err != nil {
		return err
	}
	SetService(next)
	return nil
}

func Models() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{commonmodule.NewModelRegInfo(&model.Conversation{})}
}

func Routes() commonmodule.HttpRouteGroup {
	return conversationhttp.NewConversationAPI(Service()).Routes()
}

func AdminRoutes() commonmodule.HttpRouteGroup {
	return conversationhttp.NewConversationAPI(Service()).AdminRoutes()
}
