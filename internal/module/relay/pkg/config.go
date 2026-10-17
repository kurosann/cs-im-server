package relay

import "sync"

import (
	model "cs-im-server/internal/module/relay/internal/model"
	service "cs-im-server/internal/module/relay/internal/service"
)

type (
	Channel              = model.Channel
	Webhook              = model.Webhook
	AIConfig             = model.AIConfig
	AIExecution          = model.AIExecution
	Delivery             = model.Delivery
	RelayService         = service.RelayService
	CreateChannelRequest = service.CreateChannelRequest
	UpdateChannelRequest = service.UpdateChannelRequest
	CreateWebhookRequest = service.CreateWebhookRequest
	UpdateWebhookRequest = service.UpdateWebhookRequest
)

var serviceMu sync.RWMutex
var relayService *RelayService

func SetService(next *RelayService) {
	serviceMu.Lock()
	defer serviceMu.Unlock()
	relayService = next
}

func Service() *RelayService {
	serviceMu.RLock()
	defer serviceMu.RUnlock()
	if relayService == nil {
		panic("config service is not initialized")
	}
	return relayService
}
