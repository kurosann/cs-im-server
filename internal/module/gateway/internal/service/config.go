package service

import (
	"context"
	"time"

	"vane-common/pkg/idutil"
)

func (s *Service) CreateChannel(_ context.Context, req CreateChannelRequest) (Channel, error) {
	if req.Name == "" {
		return Channel{}, badRequest("name is required")
	}
	if req.Type == "" {
		return Channel{}, badRequest("type is required")
	}
	now := time.Now().UTC()
	channel := &Channel{
		ID:        "chn_" + idutil.GenShortId(),
		Name:      req.Name,
		Type:      req.Type,
		Endpoint:  req.Endpoint,
		Config:    cloneMap(req.Config),
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels[channel.ID] = channel
	result := *channel
	s.publishLocked("channel.created", channel.ID, result)
	return result, nil
}

func (s *Service) ListChannels(_ context.Context) []Channel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	channels := make([]Channel, 0, len(s.channels))
	for _, channel := range s.channels {
		channels = append(channels, *channel)
	}
	return channels
}

func (s *Service) UpdateChannel(_ context.Context, id string, req UpdateChannelRequest) (Channel, error) {
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
	channel.UpdatedAt = now
	result := *channel
	s.publishLocked("channel.updated", id, result)
	return result, nil
}

func (s *Service) SetChannelStatus(_ context.Context, id string, enabled bool) (Channel, error) {
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
	channel.UpdatedAt = now
	result := *channel
	s.publishLocked("channel.status_changed", id, result)
	return result, nil
}

func (s *Service) TestChannel(_ context.Context, id string) (Channel, error) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	channel, ok := s.channels[id]
	if !ok {
		return Channel{}, notFound("channel %s not found", id)
	}
	channel.LastTestedAt = now
	channel.UpdatedAt = now
	result := *channel
	s.publishLocked("channel.tested", id, result)
	return result, nil
}

func (s *Service) CreateWebhook(_ context.Context, req CreateWebhookRequest) (Webhook, error) {
	if req.Name == "" {
		return Webhook{}, badRequest("name is required")
	}
	if req.URL == "" {
		return Webhook{}, badRequest("url is required")
	}
	now := time.Now().UTC()
	webhook := &Webhook{
		ID:        "wh_" + idutil.GenShortId(),
		Name:      req.Name,
		URL:       req.URL,
		Events:    cloneStrings(req.Events),
		Secret:    "whsec_" + idutil.GenId(18),
		Status:    "disabled",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if req.IsActive {
		webhook.Status = "active"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.webhooks[webhook.ID] = webhook
	result := *webhook
	s.publishLocked("webhook.created", webhook.ID, result)
	return result, nil
}

func (s *Service) ListWebhooks(_ context.Context) []Webhook {
	s.mu.RLock()
	defer s.mu.RUnlock()
	webhooks := make([]Webhook, 0, len(s.webhooks))
	for _, webhook := range s.webhooks {
		webhooks = append(webhooks, *webhook)
	}
	return webhooks
}

func (s *Service) GetWebhook(_ context.Context, id string) (Webhook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	webhook, ok := s.webhooks[id]
	if !ok {
		return Webhook{}, notFound("webhook %s not found", id)
	}
	return *webhook, nil
}

func (s *Service) UpdateWebhook(_ context.Context, id string, req UpdateWebhookRequest) (Webhook, error) {
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
	webhook.UpdatedAt = now
	result := *webhook
	s.publishLocked("webhook.updated", id, result)
	return result, nil
}

func (s *Service) TestWebhook(_ context.Context, id string) (Webhook, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	webhook, ok := s.webhooks[id]
	if !ok {
		return Webhook{}, notFound("webhook %s not found", id)
	}
	return *webhook, nil
}

func (s *Service) SetWebhookStatus(_ context.Context, id string, enabled bool) (Webhook, error) {
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
	webhook.UpdatedAt = now
	result := *webhook
	s.publishLocked("webhook.status_changed", id, result)
	return result, nil
}

func (s *Service) DeleteWebhook(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.webhooks[id]; !ok {
		return notFound("webhook %s not found", id)
	}
	delete(s.webhooks, id)
	s.publishLocked("webhook.deleted", id, nil)
	return nil
}

func (s *Service) GetAIConfig(_ context.Context) AIConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.aiConfig
}

func (s *Service) UpdateAIConfig(_ context.Context, req AIConfig) AIConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aiConfig = req
	s.publishLocked("ai.config_updated", "", s.aiConfig)
	return s.aiConfig
}

func (s *Service) ListAIExecutions(_ context.Context) []AIExecution {
	s.mu.RLock()
	defer s.mu.RUnlock()
	executions := make([]AIExecution, 0, len(s.aiExecutions))
	for _, execution := range s.aiExecutions {
		executions = append(executions, *execution)
	}
	return executions
}
