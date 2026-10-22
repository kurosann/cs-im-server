package grpc

import (
	"context"
	"time"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/grpcapi"
	gatewayv1 "github.com/kurosann/cs-im-sdk/gateway/v1"
)

type ChannelGatewayGRPCAPI struct {
	gatewayv1.UnimplementedChannelGatewayServiceServer
	Accounts      *iampkg.AccountService
	Apps          gatewaypkg.AppAuthorizer
	Conversations gatewaypkg.ConversationIngress
	Messages      gatewaypkg.MessageIngress
	Realtime      gatewaypkg.RealtimeSubscriber
}

func NewChannelGatewayGRPCAPI(accountService *iampkg.AccountService, appService gatewaypkg.AppAuthorizer, conversationService gatewaypkg.ConversationIngress, messageService gatewaypkg.MessageIngress, realtimeService gatewaypkg.RealtimeSubscriber) *ChannelGatewayGRPCAPI {
	return &ChannelGatewayGRPCAPI{
		Accounts:      accountService,
		Apps:          appService,
		Conversations: conversationService,
		Messages:      messageService,
		Realtime:      realtimeService,
	}
}

func (a *ChannelGatewayGRPCAPI) authorize(ctx context.Context, appID string) error {
	principal, err := a.Accounts.Authenticate(ctx, grpcapi.AccountToken(ctx))
	if err != nil {
		return grpcapi.Error(err)
	}
	return grpcapi.Error(a.Apps.AuthorizeAccount(appID, principal))
}

func conversationFromService(item gatewaydto.Conversation, created bool) *gatewayv1.ConversationResponse {
	return &gatewayv1.ConversationResponse{
		ConversationId: item.ConversationKey,
		AppId:          item.AppKey,
		CustomerId:     item.CustomerKey,
		Channel:        item.Channel,
		Subject:        item.Subject,
		Status:         item.Status,
		Created:        created,
	}
}

func contentFromService(item *gatewaydto.MessageMedia) *gatewayv1.MessageContent {
	if item == nil {
		return nil
	}
	return &gatewayv1.MessageContent{
		Url:             item.URL,
		MimeType:        item.MimeType,
		FileName:        item.FileName,
		Size:            item.Size,
		Width:           int32(item.Width),
		Height:          int32(item.Height),
		DurationSeconds: int32(item.DurationSeconds),
		ThumbnailUrl:    item.ThumbnailURL,
		Checksum:        item.Checksum,
	}
}

func contentFromRequest(item *gatewayv1.MessageContent) *gatewaydto.MessageMedia {
	if item == nil {
		return nil
	}
	return &gatewaydto.MessageMedia{
		URL:             item.Url,
		MimeType:        item.MimeType,
		FileName:        item.FileName,
		Size:            item.Size,
		Width:           int(item.Width),
		Height:          int(item.Height),
		DurationSeconds: int(item.DurationSeconds),
		ThumbnailURL:    item.ThumbnailUrl,
		Checksum:        item.Checksum,
	}
}

func (a *ChannelGatewayGRPCAPI) EnsureConversation(ctx context.Context, req *gatewayv1.EnsureConversationRequest) (*gatewayv1.ConversationResponse, error) {
	if err := a.authorize(ctx, req.AppId); err != nil {
		return nil, err
	}
	conversation, created, err := a.Conversations.Ensure(ctx, req.AppId, gatewaydto.EnsureConversationRequest{
		CustomerExternalID: req.CustomerExternalId,
		CustomerName:       req.CustomerName,
		Channel:            req.Channel,
		Subject:            req.Subject,
	})
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return conversationFromService(conversation, created), nil
}

func (a *ChannelGatewayGRPCAPI) GetConversation(ctx context.Context, req *gatewayv1.GetConversationRequest) (*gatewayv1.ConversationResponse, error) {
	if err := a.authorize(ctx, req.AppId); err != nil {
		return nil, err
	}
	conversation, err := a.Conversations.Get(ctx, req.ConversationId)
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return conversationFromService(conversation, false), nil
}

func (a *ChannelGatewayGRPCAPI) messageFromService(item gatewaydto.Message) *gatewayv1.Message {
	var source *gatewayv1.MessageSource
	if item.Source != nil {
		source = &gatewayv1.MessageSource{AppId: item.Source.AppID, ExternalId: item.Source.ExternalID, SentAt: item.Source.SentAt.Format(time.RFC3339Nano)}
	}
	return &gatewayv1.Message{
		MessageId:      item.MessageKey,
		Seq:            item.Seq,
		ConversationId: item.ConversationKey,
		CustomerId:     item.CustomerKey,
		Sender:         &gatewayv1.MessageSender{Role: item.Sender.Role, Id: item.Sender.ID},
		Source:         source,
		Type:           item.Type,
		Text:           item.Text,
		Content:        contentFromService(item.Content),
		Status:         item.Status,
		ReceivedAt:     item.ReceivedAt,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func (a *ChannelGatewayGRPCAPI) SendMessage(ctx context.Context, req *gatewayv1.SendMessageRequest) (*gatewayv1.MessageResponse, error) {
	if err := a.authorize(ctx, req.AppId); err != nil {
		return nil, err
	}
	request := gatewaydto.CreateMessageRequest{
		ConversationKey: req.ConversationId,
		CustomerID:      req.CustomerId,
		Sender:          gatewaydto.MessageSender{Role: req.Sender.GetRole(), ID: req.Sender.GetId()},
		Type:            req.Type,
		Text:            req.Text,
		Content:         contentFromRequest(req.Content),
	}
	if req.Source != nil {
		request.Source = gatewaydto.MessageSource{AppID: req.Source.AppId, ExternalID: req.Source.ExternalId}
		if req.Source.SentAt != "" {
			sentAt, err := time.Parse(time.RFC3339Nano, req.Source.SentAt)
			if err != nil {
				return nil, grpcapi.InvalidArgument("source.sentAt must use RFC3339")
			}
			request.Source.SentAt = &sentAt
		}
	}
	message, err := a.Messages.CreateChannelMessage(ctx, req.AppId, request)
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return &gatewayv1.MessageResponse{Message: a.messageFromService(message)}, nil
}

func (a *ChannelGatewayGRPCAPI) ListMessages(ctx context.Context, req *gatewayv1.ListMessagesRequest) (*gatewayv1.ListMessagesResponse, error) {
	if err := a.authorize(ctx, req.AppId); err != nil {
		return nil, err
	}
	page, err := a.Messages.History(ctx, req.AppId, req.ConversationId, gatewaydto.MessageHistoryFilter{
		AfterSeq:  req.AfterSeq,
		BeforeSeq: req.BeforeSeq,
		Limit:     int(req.Limit),
		Order:     req.Order,
	})
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	response := &gatewayv1.ListMessagesResponse{
		NextAfterSeq:  page.NextAfterSeq,
		NextBeforeSeq: page.NextBeforeSeq,
		HasMore:       page.HasMore,
		Items:         make([]*gatewayv1.Message, 0, len(page.Items)),
	}
	for _, item := range page.Items {
		response.Items = append(response.Items, a.messageFromService(item))
	}
	return response, nil
}

func (a *ChannelGatewayGRPCAPI) StreamMessages(req *gatewayv1.StreamMessagesRequest, stream gatewayv1.ChannelGatewayService_StreamMessagesServer) error {
	if err := a.authorize(stream.Context(), req.AppId); err != nil {
		return err
	}
	events, closeFn, err := a.Realtime.Subscribe(stream.Context())
	if err != nil {
		return grpcapi.Error(err)
	}
	defer closeFn()
	a.Apps.MarkConnected(req.AppId)
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if !a.Messages.ChannelEventAllowed(req.AppId, event) {
				continue
			}
			message, ok := event.Data.(gatewaydto.Message)
			if !ok {
				continue
			}
			if err := stream.Send(&gatewayv1.MessageEventResponse{Message: a.messageFromService(message)}); err != nil {
				return err
			}
		}
	}
}

var _ gatewayv1.ChannelGatewayServiceServer = (*ChannelGatewayGRPCAPI)(nil)
