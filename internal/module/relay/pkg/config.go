package relay

import (
	relaydto "cs-im-server/internal/module/relay/dto"
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
	CreateChannelRequest = relaydto.CreateChannelRequest
	UpdateChannelRequest = relaydto.UpdateChannelRequest
	CreateWebhookRequest = relaydto.CreateWebhookRequest
	UpdateWebhookRequest = relaydto.UpdateWebhookRequest
)

func SetService(next *RelayService) {
	service.SetGlobalService(next)
}

func Service() *RelayService {
	return service.GlobalService()
}
