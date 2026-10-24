package service

import (
	"time"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"cs-im-server/internal/module/gateway/internal/model"
)

func appToDTO(app *model.App) gatewaydto.App {
	return gatewaydto.App{
		AppKey:          app.AppKey,
		TenantID:        app.TenantID,
		ServiceKey:      app.ServiceKey,
		DisplayName:     app.DisplayName,
		Kind:            app.Kind,
		Version:         app.Version,
		Capabilities:    cloneStrings(app.Capabilities),
		Status:          app.Status,
		LeaseTTLSeconds: app.LeaseTTLSeconds,
		LastHeartbeatAt: app.LastHeartbeatAt,
		LeaseExpiresAt:  app.LeaseExpiresAt,
	}
}

func customerToDTO(customer *model.Customer) gatewaydto.Customer {
	identities := make([]gatewaydto.CustomerIdentity, 0, len(customer.Identities))
	for _, identity := range customer.Identities {
		identities = append(identities, gatewaydto.CustomerIdentity{
			Type:        identity.Type,
			ExternalID:  identity.ExternalID,
			DisplayName: identity.DisplayName,
			CreatedAt:   identity.CreatedAt,
		})
	}
	return gatewaydto.Customer{
		CustomerKey: customer.CustomerKey,
		Name:        customer.Name,
		Channel:     customer.Channel,
		Status:      customer.Status,
		Identities:  identities,
		CreatedAt:   customer.BaseModel.CreateAt.Format(time.RFC3339Nano),
		UpdatedAt:   customer.BaseModel.UpdateAt.Format(time.RFC3339Nano),
	}
}

func conversationToDTO(conversation *model.Conversation) gatewaydto.Conversation {
	return gatewaydto.Conversation{
		ConversationKey: conversation.ConversationKey,
		AppKey:          conversation.AppKey,
		CustomerKey:     conversation.CustomerKey,
		Channel:         conversation.Channel,
		Subject:         conversation.Subject,
		Status:          conversation.Status,
		Assignee:        conversation.Assignee,
		ForwardedTo:     conversation.ForwardedTo,
		CloseReason:     conversation.CloseReason,
		ClosedAt:        conversation.ClosedAt,
		Metadata:        conversation.Metadata,
		ReceivedAt:      conversation.BaseModel.CreateAt.Format(time.RFC3339Nano),
		CreatedAt:       conversation.BaseModel.CreateAt.Format(time.RFC3339Nano),
		UpdatedAt:       conversation.BaseModel.UpdateAt.Format(time.RFC3339Nano),
	}
}

func messageToDTO(message *model.Message) gatewaydto.Message {
	var source *gatewaydto.MessageSource
	if message.SourceAppKey != "" || message.SourceExternalID != "" {
		source = &gatewaydto.MessageSource{
			AppID:      message.SourceAppKey,
			ExternalID: message.SourceExternalID,
			SentAt:     message.SourceSentAt,
		}
	}
	var content *gatewaydto.MessageMedia
	if message.Media != nil {
		content = &gatewaydto.MessageMedia{
			URL:             message.Media.URL,
			MimeType:        message.Media.MimeType,
			FileName:        message.Media.FileName,
			Size:            message.Media.Size,
			Width:           message.Media.Width,
			Height:          message.Media.Height,
			DurationSeconds: message.Media.DurationSeconds,
			ThumbnailURL:    message.Media.ThumbnailURL,
			Checksum:        message.Media.Checksum,
		}
	}
	return gatewaydto.Message{
		MessageKey:      message.MessageKey,
		Seq:             message.Seq,
		ConversationKey: message.ConversationKey,
		CustomerKey:     message.CustomerKey,
		Sender:          gatewaydto.MessageSender{Role: gatewaydto.MessageSenderRole(message.SenderRole), ID: message.SenderID},
		Source:          source,
		Type:            gatewaydto.MessageType(message.Type),
		Text:            message.Text,
		Content:         content,
		Status:          gatewaydto.MessageStatus(message.Status),
		ReceivedAt:      message.CreatedAt().Format(time.RFC3339Nano),
		CreatedAt:       message.CreatedAt().Format(time.RFC3339Nano),
		UpdatedAt:       message.BaseModel.UpdateAt.Format(time.RFC3339Nano),
	}
}

func messageToDTOResult(message model.Message, err error) (gatewaydto.Message, error) {
	if err != nil {
		return gatewaydto.Message{}, err
	}
	return messageToDTO(&message), nil
}

func messageMediaToModel(item *gatewaydto.MessageMedia) *model.MessageMedia {
	if item == nil {
		return nil
	}
	return &model.MessageMedia{
		URL:             item.URL,
		MimeType:        item.MimeType,
		FileName:        item.FileName,
		Size:            item.Size,
		Width:           item.Width,
		Height:          item.Height,
		DurationSeconds: item.DurationSeconds,
		ThumbnailURL:    item.ThumbnailURL,
		Checksum:        item.Checksum,
	}
}
