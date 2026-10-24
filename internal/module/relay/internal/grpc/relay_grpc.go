package grpc

import (
	"context"
	relaydto "cs-im-server/internal/module/relay/dto"
	"time"

	iampkg "cs-im-server/internal/module/iam/pkg/iam"
	relayService "cs-im-server/internal/module/relay/internal/service"
	"cs-im-server/pkg/grpcapi"
	relayv1 "github.com/kurosann/cs-im-internal-sdk/relay/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

type RelayChannelGRPCAPI struct {
	relayv1.UnimplementedRelayChannelServiceServer
	Accounts *iampkg.AccountService
	Service  *relayService.RelayService
}

func NewRelayChannelGRPCAPI(accounts *iampkg.AccountService, relaySvc *relayService.RelayService) *RelayChannelGRPCAPI {
	return &RelayChannelGRPCAPI{Accounts: accounts, Service: relaySvc}
}

func (a *RelayChannelGRPCAPI) authorize(ctx context.Context) error {
	_, err := a.Accounts.Authenticate(ctx, grpcapi.AccountToken(ctx))
	return grpcapi.Error(err)
}

func channelToProto(channel relayService.Channel) (*relayv1.Channel, error) {
	config, err := structpb.NewStruct(channel.Config)
	if err != nil {
		return nil, grpcapi.InvalidArgument("channel config is invalid")
	}
	result := &relayv1.Channel{
		ChannelId: channel.ChannelKey,
		Name:      channel.Name,
		Type:      channel.Type,
		Endpoint:  channel.Endpoint,
		Config:    config.GetFields(),
		Status:    channel.Status,
		CreatedAt: channel.BaseModel.CreateAt.Format(time.RFC3339Nano),
		UpdatedAt: channel.BaseModel.UpdateAt.Format(time.RFC3339Nano),
	}
	if channel.LastTestedAt != nil {
		result.LastTestedAt = channel.LastTestedAt.Format(time.RFC3339Nano)
	}
	return result, nil
}

func configFromProto(values map[string]*structpb.Value) map[string]any {
	if values == nil {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value.AsInterface()
	}
	return result
}

func (a *RelayChannelGRPCAPI) CreateChannel(ctx context.Context, req *relayv1.CreateChannelRequest) (*relayv1.ChannelResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	channel, err := a.Service.CreateChannel(ctx, relaydto.CreateChannelRequest{
		Name:     req.GetName(),
		Type:     req.GetType(),
		Endpoint: req.GetEndpoint(),
		Config:   configFromProto(req.GetConfig()),
	})
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	item, err := channelToProto(channel)
	if err != nil {
		return nil, err
	}
	return &relayv1.ChannelResponse{Channel: item}, nil
}

func (a *RelayChannelGRPCAPI) ListChannels(ctx context.Context, _ *relayv1.ListChannelsRequest) (*relayv1.ListChannelsResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	channels := a.Service.ListChannels(ctx)
	items := make([]*relayv1.Channel, 0, len(channels))
	for _, channel := range channels {
		item, err := channelToProto(channel)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return &relayv1.ListChannelsResponse{Items: items}, nil
}

func (a *RelayChannelGRPCAPI) GetChannel(ctx context.Context, req *relayv1.GetChannelRequest) (*relayv1.ChannelResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	channel, err := a.Service.GetChannel(ctx, req.GetChannelId())
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	item, err := channelToProto(channel)
	if err != nil {
		return nil, err
	}
	return &relayv1.ChannelResponse{Channel: item}, nil
}

func (a *RelayChannelGRPCAPI) UpdateChannel(ctx context.Context, req *relayv1.UpdateChannelRequest) (*relayv1.ChannelResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	channel, err := a.Service.UpdateChannel(ctx, req.GetChannelId(), relaydto.UpdateChannelRequest{
		Name:     req.GetName(),
		Endpoint: req.GetEndpoint(),
		Config:   configFromProto(req.GetConfig()),
	})
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	item, err := channelToProto(channel)
	if err != nil {
		return nil, err
	}
	return &relayv1.ChannelResponse{Channel: item}, nil
}

func (a *RelayChannelGRPCAPI) SetChannelStatus(ctx context.Context, req *relayv1.SetChannelStatusRequest) (*relayv1.ChannelResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	channel, err := a.Service.SetChannelStatus(ctx, req.GetChannelId(), req.GetEnabled())
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	item, err := channelToProto(channel)
	if err != nil {
		return nil, err
	}
	return &relayv1.ChannelResponse{Channel: item}, nil
}

func (a *RelayChannelGRPCAPI) TestChannel(ctx context.Context, req *relayv1.TestChannelRequest) (*relayv1.ChannelResponse, error) {
	if err := a.authorize(ctx); err != nil {
		return nil, err
	}
	channel, err := a.Service.TestChannel(ctx, req.GetChannelId())
	if err != nil {
		return nil, grpcapi.Error(err)
	}
	item, err := channelToProto(channel)
	if err != nil {
		return nil, err
	}
	return &relayv1.ChannelResponse{Channel: item}, nil
}
