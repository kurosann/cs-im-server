package service

import (
	configmodel "cs-im-server/internal/module/relay/internal/model"
)

type Channel = configmodel.Channel
type Webhook = configmodel.Webhook
type AIConfig = configmodel.AIConfig
type AIExecution = configmodel.AIExecution
type Delivery = configmodel.Delivery

type RealtimePublisher interface {
	Publish(eventType, targetID string, payload any)
}
