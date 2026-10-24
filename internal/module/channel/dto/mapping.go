package dto

import (
	gatewaydto "cs-im-server/internal/module/gateway/dto"
)

func EnsureConversationToGateway(req EnsureConversationRequest) gatewaydto.EnsureConversationRequest {
	return gatewaydto.EnsureConversationRequest{
		CustomerExternalID: req.CustomerExternalID,
		CustomerName:       req.CustomerName,
		Channel:            req.Channel,
		Subject:            req.Subject,
		Metadata:           req.Metadata,
	}
}

func HeartbeatToGateway(req HeartbeatRequest) gatewaydto.HeartbeatRequest {
	return gatewaydto.HeartbeatRequest{AppID: req.AppID}
}

func MessageMediaToGateway(item *MessageMedia) *gatewaydto.MessageMedia {
	if item == nil {
		return nil
	}
	return &gatewaydto.MessageMedia{
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

func CreateMessageRequestToGateway(req CreateMessageRequest) gatewaydto.CreateMessageRequest {
	return gatewaydto.CreateMessageRequest{
		ConversationKey: req.ConversationID,
		CustomerID:      req.CustomerID,
		Sender:          gatewaydto.MessageSender{Role: gatewaydto.MessageSenderRole(req.Sender.Role), ID: req.Sender.ID},
		Source:          MessageSourceToGateway(req.Source),
		Type:            gatewaydto.MessageType(req.Type),
		Text:            req.Text,
		Content:         MessageMediaToGateway(req.Content),
		Status:          gatewaydto.MessageStatus(req.Status),
	}
}

func MessageSourceToGateway(item MessageSource) gatewaydto.MessageSource {
	return gatewaydto.MessageSource{
		AppID:      item.AppID,
		ExternalID: item.ExternalID,
		SentAt:     item.SentAt,
	}
}

func BatchCreateMessagesToGateway(req BatchCreateMessagesRequest) gatewaydto.BatchCreateMessagesRequest {
	items := make([]gatewaydto.CreateMessageRequest, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, CreateMessageRequestToGateway(item))
	}
	return gatewaydto.BatchCreateMessagesRequest{Items: items}
}

func HistoryRequestFromGateway(item gatewaydto.HistoryRequest) HistoryRequest {
	return HistoryRequest{
		RequestID:          item.RequestID,
		AppID:              item.AppID,
		ConversationID:     item.ConversationID,
		CustomerExternalID: item.CustomerExternalID,
		Status:             HistoryRequestStatus(item.Status),
		CreatedAt:          item.CreatedAt,
		CompletedAt:        item.CompletedAt,
	}
}

func UpdateHistoryRequestStatusToGateway(req UpdateHistoryRequestStatusInput) gatewaydto.CompleteHistoryRequestInput {
	return gatewaydto.CompleteHistoryRequestInput{
		RequestID: req.RequestID,
		Status:    gatewaydto.HistoryRequestStatus(req.Status),
	}
}
