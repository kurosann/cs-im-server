package gatewaybridge

import (
	"context"
	"strings"

	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	relayservice "cs-im-server/internal/module/relay/internal/service"
)

type bridge struct{}

func Register(service *relayservice.RelayService) {
	if service == nil {
		return
	}
	service.SetInboundGateway(bridge{})
	gatewaypkg.SetCustomerMessageObserver(func(message gatewaypkg.Message) {
		service.EnqueueInbound(relayservice.InboundMessage{
			ID:             message.MessageKey,
			ConversationID: message.ConversationKey,
			Role:           message.SenderRole,
			Type:           message.Type,
			Text:           message.Text,
		})
	})
}

func (bridge) LookupConversation(ctx context.Context, conversationID string) (relayservice.ConversationRef, error) {
	conversation, err := gatewaypkg.GetConversationIngress().Get(ctx, conversationID)
	if err != nil {
		return relayservice.ConversationRef{}, err
	}
	return relayservice.ConversationRef{
		AppID:      conversation.AppKey,
		CustomerID: conversation.CustomerKey,
		Subject:    strings.TrimSpace(conversation.Subject),
	}, nil
}

func (bridge) RecentMessages(ctx context.Context, conversationID string, limit int) ([]relayservice.TranscriptMessage, error) {
	messages, err := gatewaypkg.GetMessageEgress().RecentMessages(ctx, conversationID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]relayservice.TranscriptMessage, 0, len(messages))
	for _, message := range messages {
		out = append(out, relayservice.TranscriptMessage{
			ID:   message.MessageKey,
			Role: message.Sender.Role,
			Type: message.Type,
			Text: message.Text,
			Seq:  message.Seq,
		})
	}
	return out, nil
}

func (bridge) SendAIReply(ctx context.Context, conversationID, employeeID, text string) error {
	_, err := gatewaypkg.GetMessageEgress().CreateAIReply(ctx, conversationID, employeeID, text)
	return err
}
