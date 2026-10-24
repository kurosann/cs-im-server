package service

import (
	"context"
	"sync"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

	relaydto "cs-im-server/internal/module/relay/dto"
)

type RelayService struct {
	mu               sync.RWMutex
	channelAccessor  dataaccess.DataAccessor[*Channel]
	webhookAccessor  dataaccess.DataAccessor[*Webhook]
	aiConfigAccessor dataaccess.DataAccessor[*AIConfig]
	employees        dataaccess.DataAccessor[*AIEmployee]
	bindings         dataaccess.DataAccessor[*AIEmployeeBinding]
	executions       dataaccess.DataAccessor[*AIEmployeeExecution]
	apps             dataaccess.DataAccessor[*businessAppRow]
	knowledgeGroups  dataaccess.DataAccessor[*KnowledgeGroup]
	knowledgeBases   dataaccess.DataAccessor[*KnowledgeBase]
	knowledgeDocs    dataaccess.DataAccessor[*KnowledgeDoc]
	userTenants      dataaccess.DataAccessor[*iamUserRow]
	completer        CompletionClient
	inbound          InboundGateway
	inboundMu        sync.Mutex
	inboundPending   map[string]*inboundBatch
	inboundLanes     map[string]*sync.Mutex
	aiConfigID       int64
	realtime         RealtimePublisher
}

type RuntimeDeps struct {
	Realtime  RealtimePublisher
	Inbound   InboundGateway
	Completer CompletionClient
}

func NewRelayService(deps RuntimeDeps) *RelayService {
	return &RelayService{
		completer: deps.Completer,
		inbound:   deps.Inbound,
		realtime:  deps.Realtime,
	}
}

func (s *RelayService) CreateChannel(_ context.Context, req relaydto.CreateChannelRequest) (Channel, error) {
	if req.Name == "" {
		return Channel{}, badRequest("name is required")
	}
	if req.Type == "" {
		return Channel{}, badRequest("type is required")
	}
	channel := &Channel{ChannelKey: "chn_" + idutil.GenShortId(), Name: req.Name, Type: req.Type, Endpoint: req.Endpoint, Config: cloneMap(req.Config), Status: "active"}
	if err := s.channelAccessor.Save(context.Background(), channel); err != nil {
		return Channel{}, err
	}
	result := *channel
	s.realtime.Publish("channel.created", channel.ChannelKey, result)
	return result, nil
}

func (s *RelayService) ListChannels(_ context.Context) []Channel {
	channels, err := s.listChannels(context.Background())
	if err != nil {
		return []Channel{}
	}
	return channels
}

func (s *RelayService) GetChannel(_ context.Context, id string) (Channel, error) {
	channel, err := s.getChannelByKey(context.Background(), id)
	if err != nil {
		return Channel{}, notFound("channel %s not found", id)
	}
	return *channel, nil
}

func (s *RelayService) UpdateChannel(_ context.Context, id string, req relaydto.UpdateChannelRequest) (Channel, error) {
	now := time.Now().UTC()
	channel, err := s.getChannelByKey(context.Background(), id)
	if err != nil {
		return Channel{}, notFound("channel %s not found", id)
	}
	if req.Name != "" {
		channel.Name = req.Name
	}
	if req.Type != "" {
		channel.Type = req.Type
	}
	if req.Endpoint != "" {
		channel.Endpoint = req.Endpoint
	}
	if req.Config != nil {
		channel.Config = cloneMap(req.Config)
	}
	channel.UpdateAt = now
	if err := s.channelAccessor.Save(context.Background(), channel); err != nil {
		return Channel{}, err
	}
	result := *channel
	s.realtime.Publish("channel.updated", id, result)
	return result, nil
}

func (s *RelayService) SetChannelStatus(_ context.Context, id string, enabled bool) (Channel, error) {
	now := time.Now().UTC()
	status := "disabled"
	if enabled {
		status = "active"
	}
	channel, err := s.getChannelByKey(context.Background(), id)
	if err != nil {
		return Channel{}, notFound("channel %s not found", id)
	}
	channel.Status = status
	channel.UpdateAt = now
	if err := s.channelAccessor.Save(context.Background(), channel); err != nil {
		return Channel{}, err
	}
	result := *channel
	s.realtime.Publish("channel.status_changed", id, result)
	return result, nil
}

func (s *RelayService) TestChannel(_ context.Context, id string) (Channel, error) {
	now := time.Now().UTC()
	channel, err := s.getChannelByKey(context.Background(), id)
	if err != nil {
		return Channel{}, notFound("channel %s not found", id)
	}
	channel.LastTestedAt = &now
	channel.UpdateAt = now
	if err := s.channelAccessor.Save(context.Background(), channel); err != nil {
		return Channel{}, err
	}
	result := *channel
	s.realtime.Publish("channel.tested", id, result)
	return result, nil
}

func (s *RelayService) CreateWebhook(_ context.Context, req relaydto.CreateWebhookRequest) (Webhook, error) {
	if req.Name == "" {
		return Webhook{}, badRequest("name is required")
	}
	if req.URL == "" {
		return Webhook{}, badRequest("url is required")
	}
	webhook := &Webhook{WebhookKey: "wh_" + idutil.GenShortId(), Name: req.Name, URL: req.URL, Events: cloneStrings(req.Events), Secret: "whsec_" + idutil.GenId(18), Status: "disabled"}
	if req.IsActive {
		webhook.Status = "active"
	}
	if err := s.webhookAccessor.Save(context.Background(), webhook); err != nil {
		return Webhook{}, err
	}
	result := *webhook
	s.realtime.Publish("webhook.created", webhook.WebhookKey, result)
	return result, nil
}

func (s *RelayService) ListWebhooks(_ context.Context) []Webhook {
	webhooks, err := s.listWebhooks(context.Background())
	if err != nil {
		return []Webhook{}
	}
	return webhooks
}

func (s *RelayService) GetWebhook(_ context.Context, id string) (Webhook, error) {
	webhook, err := s.getWebhookByKey(context.Background(), id)
	if err != nil {
		return Webhook{}, notFound("webhook %s not found", id)
	}
	return *webhook, nil
}

func (s *RelayService) UpdateWebhook(_ context.Context, id string, req relaydto.UpdateWebhookRequest) (Webhook, error) {
	now := time.Now().UTC()
	webhook, err := s.getWebhookByKey(context.Background(), id)
	if err != nil {
		return Webhook{}, notFound("webhook %s not found", id)
	}
	if req.Name != "" {
		webhook.Name = req.Name
	}
	if req.URL != "" {
		webhook.URL = req.URL
	}
	if req.Events != nil {
		webhook.Events = cloneStrings(req.Events)
	}
	if req.IsActive {
		webhook.Status = "active"
	}
	webhook.UpdateAt = now
	if err := s.webhookAccessor.Save(context.Background(), webhook); err != nil {
		return Webhook{}, err
	}
	result := *webhook
	s.realtime.Publish("webhook.updated", id, result)
	return result, nil
}

func (s *RelayService) TestWebhook(_ context.Context, id string) (Webhook, error) {
	webhook, err := s.getWebhookByKey(context.Background(), id)
	if err != nil {
		return Webhook{}, notFound("webhook %s not found", id)
	}
	return *webhook, nil
}

func (s *RelayService) SetWebhookStatus(_ context.Context, id string, enabled bool) (Webhook, error) {
	now := time.Now().UTC()
	status := "disabled"
	if enabled {
		status = "active"
	}
	webhook, err := s.getWebhookByKey(context.Background(), id)
	if err != nil {
		return Webhook{}, notFound("webhook %s not found", id)
	}
	webhook.Status = status
	webhook.UpdateAt = now
	if err := s.webhookAccessor.Save(context.Background(), webhook); err != nil {
		return Webhook{}, err
	}
	result := *webhook
	s.realtime.Publish("webhook.status_changed", id, result)
	return result, nil
}

func (s *RelayService) DeleteWebhook(_ context.Context, id string) error {
	webhook, err := s.getWebhookByKey(context.Background(), id)
	if err != nil {
		return notFound("webhook %s not found", id)
	}
	if err := s.webhookAccessor.Delete(context.Background(), webhook); err != nil {
		return err
	}
	s.realtime.Publish("webhook.deleted", id, nil)
	return nil
}

func (s *RelayService) GetAIConfig(_ context.Context) AIConfig {
	item, err := s.getAIConfig(context.Background())
	if err != nil {
		return AIConfig{}
	}
	if item != nil {
		return *item
	}
	return AIConfig{Prompt: "You are a helpful customer service assistant.", HistoryMessageCount: 20, AutoReplyEnabled: true, ReportingEnabled: true}
}

func (s *RelayService) UpdateAIConfig(ctx context.Context, req AIConfig) AIConfig {
	existing, err := s.getAIConfig(ctx)
	if err != nil {
		return AIConfig{}
	}
	item := req
	if existing != nil {
		item = *existing
	}
	if err := s.aiConfigAccessor.Save(ctx, &item); err != nil {
		if existing != nil {
			return *existing
		}
		return AIConfig{}
	}
	s.realtime.Publish("ai.config_updated", "", item)
	return item
}

func (s *RelayService) ListAIExecutions(_ context.Context) []AIExecution {
	items, err := s.listAIExecutions(context.Background())
	if err != nil {
		return []AIExecution{}
	}
	return items
}

func (s *RelayService) Load() error {
	return s.backfillEmployeeTenants(context.Background())
}
func (s *RelayService) listChannels(ctx context.Context) ([]Channel, error) {
	var found []*Channel
	if err := s.channelAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(
		dataaccess.Order("create_at", dataaccess.OrderTypeAsc),
	)); err != nil {
		return nil, err
	}
	channels := make([]Channel, 0, len(found))
	for _, item := range found {
		channels = append(channels, *item)
	}
	return channels, nil
}

func (s *RelayService) getChannelByKey(ctx context.Context, id string) (*Channel, error) {
	var found []*Channel
	if err := s.channelAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(
		dataaccess.Eq("channel_key", id),
		dataaccess.Limit(1),
	)); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, dataaccess.ErrDataNotFound
	}
	return found[0], nil
}

func (s *RelayService) listWebhooks(ctx context.Context) ([]Webhook, error) {
	var found []*Webhook
	if err := s.webhookAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(
		dataaccess.Order("create_at", dataaccess.OrderTypeAsc),
	)); err != nil {
		return nil, err
	}
	webhooks := make([]Webhook, 0, len(found))
	for _, item := range found {
		webhooks = append(webhooks, *item)
	}
	return webhooks, nil
}

func (s *RelayService) getWebhookByKey(ctx context.Context, id string) (*Webhook, error) {
	var found []*Webhook
	if err := s.webhookAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(
		dataaccess.Eq("webhook_key", id),
		dataaccess.Limit(1),
	)); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, dataaccess.ErrDataNotFound
	}
	return found[0], nil
}

func (s *RelayService) getAIConfig(ctx context.Context) (*AIConfig, error) {
	var found []*AIConfig
	if err := s.aiConfigAccessor.ListWithOption(ctx, &found, dataaccess.WithConditions(
		dataaccess.Order("create_at", dataaccess.OrderTypeAsc),
		dataaccess.Limit(1),
	)); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, nil
	}
	return found[0], nil
}

func (s *RelayService) listAIExecutions(ctx context.Context) ([]AIExecution, error) {
	var found []*AIExecution
	if err := s.executions.ListWithOption(ctx, &found, dataaccess.WithConditions(
		dataaccess.Order("create_at", dataaccess.OrderTypeDesc),
		dataaccess.Limit(100),
	)); err != nil {
		return nil, err
	}
	executions := make([]AIExecution, 0, len(found))
	for _, item := range found {
		executions = append(executions, *item)
	}
	return executions, nil
}
