package gateway

import (
	apppkg "cs-im-server/internal/module/gateway/internal/app/pkg"
	conversationpkg "cs-im-server/internal/module/gateway/internal/conversation/pkg"
	customerpkg "cs-im-server/internal/module/gateway/internal/customer/pkg"
	messagepkg "cs-im-server/internal/module/gateway/internal/message/pkg"
	realtimepkg "cs-im-server/internal/module/gateway/internal/realtime/pkg"
)

type (
	App                = apppkg.App
	AppService         = apppkg.AppService
	RegisterAppRequest = apppkg.RegisterAppRequest
	HeartbeatRequest   = apppkg.HeartbeatRequest

	Customer         = customerpkg.Customer
	CustomerIdentity = customerpkg.CustomerIdentity
	CustomerService  = customerpkg.CustomerService

	Conversation               = conversationpkg.Conversation
	ConversationService        = conversationpkg.ConversationService
	CreateConversationRequest  = conversationpkg.CreateConversationRequest
	AssignConversationRequest  = conversationpkg.AssignConversationRequest
	CloseConversationRequest   = conversationpkg.CloseConversationRequest
	ForwardConversationRequest = conversationpkg.ForwardConversationRequest
	EnsureConversationRequest  = conversationpkg.EnsureConversationRequest

	Message                    = messagepkg.Message
	MessageSender              = messagepkg.MessageSender
	MessageSource              = messagepkg.MessageSource
	MessageService             = messagepkg.MessageService
	CreateMessageRequest       = messagepkg.CreateMessageRequest
	UpdateMessageStatusRequest = messagepkg.UpdateMessageStatusRequest
	MessageHistoryFilter       = messagepkg.MessageHistoryFilter
	MessageHistoryPage         = messagepkg.MessageHistoryPage

	RealtimeEvent   = realtimepkg.RealtimeEvent
	RealtimeService = realtimepkg.RealtimeService
)

func GetApp() *AppService                   { return apppkg.Service() }
func GetCustomer() *CustomerService         { return customerpkg.Service() }
func GetConversation() *ConversationService { return conversationpkg.Service() }
func GetMessage() *MessageService           { return messagepkg.Service() }
func GetRealtime() *RealtimeService         { return realtimepkg.Service() }
