package service

import (
	messagemodel "cs-im-server/internal/module/gateway/internal/message/model"

	apppkg "cs-im-server/internal/module/gateway/internal/app/pkg"
	conversationpkg "cs-im-server/internal/module/gateway/internal/conversation/pkg"
	realtimepkg "cs-im-server/internal/module/gateway/internal/realtime/pkg"
)

type Message = messagemodel.Message
type MessageSender = messagemodel.MessageSender
type MessageSource = messagemodel.MessageSource

type AppService = apppkg.AppService
type ConversationService = conversationpkg.ConversationService
type RealtimeService = realtimepkg.RealtimeService
type RealtimeEvent = realtimepkg.RealtimeEvent
