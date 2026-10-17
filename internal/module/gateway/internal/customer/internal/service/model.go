package service

import (
	customermodel "cs-im-server/internal/module/gateway/internal/customer/model"
	realtimepkg "cs-im-server/internal/module/gateway/internal/realtime/pkg"
)

type Customer = customermodel.Customer
type CustomerIdentity = customermodel.CustomerIdentity
type RealtimeService = realtimepkg.RealtimeService

type ConversationRetargeter interface {
	RetargetCustomer(sourceID, targetID string) error
}
