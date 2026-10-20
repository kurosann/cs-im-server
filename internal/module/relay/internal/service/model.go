package service

import (
	configmodel "cs-im-server/internal/module/relay/internal/model"
)

type Channel = configmodel.Channel
type Webhook = configmodel.Webhook
type AIConfig = configmodel.AIConfig
type AIExecution = configmodel.AIExecution
type AIEmployee = configmodel.AIEmployee
type AIEmployeeBinding = configmodel.AIEmployeeBinding
type AIEmployeeExecution = configmodel.AIEmployeeExecution
type HistoryDiagnostics = configmodel.HistoryDiagnostics
type Delivery = configmodel.Delivery

type RealtimePublisher interface {
	Publish(eventType, targetID string, payload any)
}
