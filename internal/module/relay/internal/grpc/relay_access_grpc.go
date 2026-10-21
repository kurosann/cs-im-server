package grpc

import (
	"context"
	"time"

	gatewaypkg "cs-im-server/internal/module/gateway/pkg"
	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	"cs-im-server/pkg/grpcapi"
	"cs-im-server/pkg/seqquery"
	relayv1 "github.com/kurosann/cs-im-internal-sdk/relay/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RelayAccessGRPCAPI struct {
	relayv1.UnimplementedRelayAccessServiceServer
	Accounts *iampkg.AccountService
	Messages gatewaypkg.MessageEgress
	Realtime gatewaypkg.RealtimeSubscriber
}

func NewRelayAccessGRPCAPI(accounts *iampkg.AccountService, messages gatewaypkg.MessageEgress, realtime gatewaypkg.RealtimeSubscriber) *RelayAccessGRPCAPI {
	return &RelayAccessGRPCAPI{Accounts: accounts, Messages: messages, Realtime: realtime}
}

func (a *RelayAccessGRPCAPI) relayPrincipal(ctx context.Context) (iampkg.Principal, error) {
	principal, err := a.Accounts.Authenticate(ctx, grpcapi.AccountToken(ctx))
	if err != nil {
		return iampkg.Principal{}, grpcapi.Error(err)
	}
	if !(principal.HasKind(iampkg.PrincipalKindRelay) ||
		(principal.Kind == iampkg.PrincipalKindHuman && principal.HasScope(iampkg.ScopeRelayAccess))) {
		return iampkg.Principal{}, status.Error(codes.PermissionDenied, "relay access is not allowed")
	}
	return principal, nil
}

func relayMessageToProto(item gatewaypkg.Message) *relayv1.RelayMessage {
	message := &relayv1.RelayMessage{
		MessageId:      item.MessageKey,
		Seq:            item.Seq,
		ConversationId: item.ConversationKey,
		CustomerId:     item.CustomerKey,
		Sender: &relayv1.RelayMessageSender{
			Role: item.SenderRole,
			Id:   item.SenderID,
		},
		Type:       item.Type,
		Text:       item.Text,
		Status:     item.Status,
		ReceivedAt: item.BaseModel.CreateAt.Format(time.RFC3339Nano),
		CreatedAt:  item.CreatedAt().Format(time.RFC3339Nano),
		UpdatedAt:  item.BaseModel.UpdateAt.Format(time.RFC3339Nano),
	}
	if item.Media != nil {
		message.Content = &relayv1.RelayMessageContent{
			Url:             item.Media.URL,
			MimeType:        item.Media.MimeType,
			FileName:        item.Media.FileName,
			Size:            item.Media.Size,
			Width:           int32(item.Media.Width),
			Height:          int32(item.Media.Height),
			DurationSeconds: int32(item.Media.DurationSeconds),
			ThumbnailUrl:    item.Media.ThumbnailURL,
			Checksum:        item.Media.Checksum,
		}
	}
	return message
}

func relayMessageFromRequest(req *relayv1.SendRelayMessageRequest) gatewaypkg.CreateMessageRequest {
	if req == nil {
		return gatewaypkg.CreateMessageRequest{}
	}
	request := gatewaypkg.CreateMessageRequest{
		ConversationKey: req.GetConversationId(),
		CustomerID:      req.GetCustomerId(),
		Sender:          gatewaypkg.MessageSender{Role: req.GetSender().GetRole(), ID: req.GetSender().GetId()},
		Type:            req.GetType(),
		Text:            req.GetText(),
	}
	if item := req.GetContent(); item != nil {
		request.Content = &gatewaypkg.MessageMedia{
			URL:             item.GetUrl(),
			MimeType:        item.GetMimeType(),
			FileName:        item.GetFileName(),
			Size:            item.GetSize(),
			Width:           int(item.GetWidth()),
			Height:          int(item.GetHeight()),
			DurationSeconds: int(item.GetDurationSeconds()),
			ThumbnailURL:    item.GetThumbnailUrl(),
			Checksum:        item.GetChecksum(),
		}
	}
	return request
}

func (a *RelayAccessGRPCAPI) SendRelayMessage(ctx context.Context, req *relayv1.SendRelayMessageRequest) (*relayv1.RelayMessageResponse, error) {
	principal, err := a.relayPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	message, err := a.Messages.CreateRelayMessage(ctx, principal.AccountKey, relayMessageFromRequest(req))
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	return &relayv1.RelayMessageResponse{Message: relayMessageToProto(message)}, nil
}

func (a *RelayAccessGRPCAPI) ListRelayMessages(ctx context.Context, req *relayv1.ListRelayMessagesRequest) (*relayv1.ListRelayMessagesResponse, error) {
	if _, err := a.relayPrincipal(ctx); err != nil {
		return nil, err
	}
	page, err := a.Messages.RelayHistory(ctx, req.GetConversationId(), seqquery.Filter{
		AfterSeq:  req.GetAfterSeq(),
		BeforeSeq: req.GetBeforeSeq(),
		Limit:     int(req.GetLimit()),
		Order:     req.GetOrder(),
	})
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	items := make([]*relayv1.RelayMessage, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, relayMessageToProto(item))
	}
	return &relayv1.ListRelayMessagesResponse{
		Items:         items,
		NextAfterSeq:  page.NextAfterSeq,
		NextBeforeSeq: page.NextBeforeSeq,
		HasMore:       page.HasMore,
	}, nil
}

func (a *RelayAccessGRPCAPI) StreamRelayMessages(req *relayv1.StreamRelayMessagesRequest, stream relayv1.RelayAccessService_StreamRelayMessagesServer) error {
	if _, err := a.relayPrincipal(stream.Context()); err != nil {
		return err
	}
	events, closeFn, err := a.Realtime.Subscribe(stream.Context())
	if err != nil {
		return grpcapi.Error(err)
	}
	defer closeFn()
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if !a.Messages.RelayEventAllowed(event) {
				continue
			}
			response := &relayv1.RelayMessageEventResponse{EventType: event.Type}
			if message, ok := event.Data.(gatewaypkg.Message); ok {
				response.Message = relayMessageToProto(message)
			}
			if err := stream.Send(response); err != nil {
				return err
			}
		}
	}
}
