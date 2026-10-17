package message

import "sync"

import (
	commonmodule "vane-common/module"

	apppkg "cs-im-server/internal/module/gateway/internal/app/pkg"
	conversationpkg "cs-im-server/internal/module/gateway/internal/conversation/pkg"
	messagehttp "cs-im-server/internal/module/gateway/internal/message/http"
	service "cs-im-server/internal/module/gateway/internal/message/internal/service"
	model "cs-im-server/internal/module/gateway/internal/message/model"
	realtimepkg "cs-im-server/internal/module/gateway/internal/realtime/pkg"
)

type (
	Message                    = model.Message
	MessageSender              = model.MessageSender
	MessageSource              = model.MessageSource
	MessageService             = service.MessageService
	CreateMessageRequest       = service.CreateMessageRequest
	UpdateMessageStatusRequest = service.UpdateMessageStatusRequest
	MessageHistoryFilter       = service.MessageHistoryFilter
	MessageHistoryPage         = service.MessageHistoryPage
)

var serviceMu sync.RWMutex
var messageService *MessageService

func SetService(next *MessageService) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	messageService = next
}

func Service() *MessageService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if messageService == nil {
		panic("message service is not initialized")
	}
	return messageService
}

func Init(apps *apppkg.AppService, conversations *conversationpkg.ConversationService, realtime *realtimepkg.RealtimeService) error {
	next := service.NewMessageService(apps, conversations, realtime)
	if err := next.Load(); err != nil {
		return err
	}
	SetService(next)
	return nil
}

func Models() []commonmodule.ModelRegInfo {
	return []commonmodule.ModelRegInfo{commonmodule.NewModelRegInfo(&model.Message{})}
}

func Routes() commonmodule.HttpRouteGroup { return messagehttp.NewMessageAPI(Service()).Routes() }

func ConversationRoutes() commonmodule.HttpRouteGroup {
	return messagehttp.NewMessageAPI(Service()).ConversationRoutes()
}

func NewConversationMessageStore() conversationpkg.MessageStore {
	return service.NewConversationMessageStore(Service())
}
