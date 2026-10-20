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
	apps             dataaccess.DataAccessor[*businessAppRow]
	channels         map[string]*Channel
	webhooks         map[string]*Webhook
	aiConfigID       int64
	aiConfig         AIConfig
	aiExecutions     []*AIExecution
	realtime         RealtimePublisher
}

func NewRelayService(realtime RealtimePublisher) *RelayService {
	return &RelayService{
		channels: make(map[string]*Channel),
		webhooks: make(map[string]*Webhook),
		aiConfig: AIConfig{Prompt: "You are a helpful customer service assistant.", HistoryMessageCount: 20, AutoReplyEnabled: true, ReportingEnabled: true},
		realtime: realtime,
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.channelAccessor.Save(context.Background(), channel); err != nil {
		return Channel{}, err
	}
	s.channels[channel.ChannelKey] = channel
	result := *channel
	s.realtime.Publish("channel.created", channel.ChannelKey, result)
	return result, nil
}

func (s *RelayService) ListChannels(_ context.Context) []Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	channels := make([]Channel, 0, len(s.channels))
	for _, channel := range s.channels {
		channels = append(channels, *channel)
	}
	return channels
}

func (s *RelayService) GetChannel(_ context.Context, id string) (Channel, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	channel, ok := s.channels[id]
	if !ok {
		return Channel{}, notFound("channel %s not found", id)
	}
	return *channel, nil
}

func (s *RelayService) UpdateChannel(_ context.Context, id string, req relaydto.UpdateChannelRequest) (Channel, error) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	channel, ok := s.channels[id]
	if !ok {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	channel, ok := s.channels[id]
	if !ok {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	channel, ok := s.channels[id]
	if !ok {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.webhookAccessor.Save(context.Background(), webhook); err != nil {
		return Webhook{}, err
	}
	s.webhooks[webhook.WebhookKey] = webhook
	result := *webhook
	s.realtime.Publish("webhook.created", webhook.WebhookKey, result)
	return result, nil
}

func (s *RelayService) ListWebhooks(_ context.Context) []Webhook {
	s.mu.RLock()
	defer s.mu.RUnlock()
	webhooks := make([]Webhook, 0, len(s.webhooks))
	for _, webhook := range s.webhooks {
		webhooks = append(webhooks, *webhook)
	}
	return webhooks
}

func (s *RelayService) GetWebhook(_ context.Context, id string) (Webhook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	webhook, ok := s.webhooks[id]
	if !ok {
		return Webhook{}, notFound("webhook %s not found", id)
	}
	return *webhook, nil
}

func (s *RelayService) UpdateWebhook(_ context.Context, id string, req relaydto.UpdateWebhookRequest) (Webhook, error) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	webhook, ok := s.webhooks[id]
	if !ok {
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	webhook, ok := s.webhooks[id]
	if !ok {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	webhook, ok := s.webhooks[id]
	if !ok {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	webhook, ok := s.webhooks[id]
	if !ok {
		return notFound("webhook %s not found", id)
	}
	if err := s.webhookAccessor.Delete(context.Background(), webhook); err != nil {
		return err
	}
	delete(s.webhooks, id)
	s.realtime.Publish("webhook.deleted", id, nil)
	return nil
}

func (s *RelayService) GetAIConfig(_ context.Context) AIConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.aiConfig
}

func (s *RelayService) UpdateAIConfig(_ context.Context, req AIConfig) AIConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := req
	item.ID = s.aiConfigID
	if err := s.aiConfigAccessor.Save(context.Background(), &item); err != nil {
		return s.aiConfig
	}
	s.aiConfigID = item.ID
	s.aiConfig = item
	s.realtime.Publish("ai.config_updated", "", s.aiConfig)
	return s.aiConfig
}

func (s *RelayService) ListAIExecutions(_ context.Context) []AIExecution {
	s.mu.RLock()
	defer s.mu.RUnlock()
	executions := make([]AIExecution, 0, len(s.aiExecutions))
	for _, execution := range s.aiExecutions {
		executions = append(executions, *execution)
	}
	return executions
}

func (s *RelayService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	channels, err := s.channelAccessor.List(context.Background())
	if err != nil {
		return err
	}
	for _, channel := range channels {
		s.channels[channel.ChannelKey] = channel
	}
	webhooks, err := s.webhookAccessor.List(context.Background())
	if err != nil {
		return err
	}
	for _, webhook := range webhooks {
		s.webhooks[webhook.WebhookKey] = webhook
	}
	aiConfigs, err := s.aiConfigAccessor.List(context.Background())
	if err != nil {
		return err
	}
	if len(aiConfigs) > 0 {
		s.aiConfig = *aiConfigs[0]
		s.aiConfigID = aiConfigs[0].ID
	}
	return nil
}
