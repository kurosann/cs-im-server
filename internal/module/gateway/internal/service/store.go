package service

import (
	"context"
	"sort"
	"sync"
	"time"

	"vane-common/pkg/idutil"
)

type RegisterAppRequest struct {
	ServiceKey      string   `json:"serviceKey"`
	DisplayName     string   `json:"displayName"`
	Kind            string   `json:"kind"`
	Version         string   `json:"version"`
	Capabilities    []string `json:"capabilities"`
	LeaseTTLSeconds int      `json:"leaseTtlSeconds"`
}

type HeartbeatRequest struct {
	AppID        string   `json:"appId"`
	InstanceID   string   `json:"instanceId"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
	TTLSeconds   int      `json:"ttlSeconds"`
}

type CreateCustomerRequest struct {
	Name         string `json:"name"`
	Channel      string `json:"channel"`
	IdentityType string `json:"identityType"`
	ExternalID   string `json:"externalId"`
	DisplayName  string `json:"displayName"`
}

type AddIdentityRequest struct {
	Type        string `json:"type"`
	ExternalID  string `json:"externalId"`
	DisplayName string `json:"displayName"`
}

type MergeCustomerRequest struct {
	SourceCustomerID string `json:"sourceCustomerId"`
}

type CreateConversationRequest struct {
	AppID      string `json:"appId"`
	CustomerID string `json:"customerId"`
	Channel    string `json:"channel"`
	Subject    string `json:"subject"`
}

type AssignConversationRequest struct {
	Assignee string `json:"assignee"`
}

type CloseConversationRequest struct {
	Reason string `json:"reason"`
}

type ForwardConversationRequest struct {
	MessageID string `json:"messageId"`
	ChannelID string `json:"channelId"`
}

type CreateMessageRequest struct {
	ConversationID string        `json:"conversationId"`
	CustomerID     string        `json:"customerId"`
	Sender         MessageSender `json:"sender"`
	Source         MessageSource `json:"source"`
	Type           string        `json:"type"`
	Text           string        `json:"text"`
}

type UpdateMessageStatusRequest struct {
	Status string `json:"status"`
}

type CreateChannelRequest struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Endpoint string         `json:"endpoint"`
	Config   map[string]any `json:"config"`
}

type UpdateChannelRequest struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Endpoint string         `json:"endpoint"`
	Config   map[string]any `json:"config"`
}

type CreateWebhookRequest struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Events   []string `json:"events"`
	IsActive bool     `json:"isActive"`
}

type UpdateWebhookRequest struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Events   []string `json:"events"`
	IsActive bool     `json:"isActive"`
}

type Service struct {
	mu              sync.RWMutex
	apps            map[string]*App
	appIDsByService map[string]string
	customers       map[string]*Customer
	conversations   map[string]*Conversation
	messages        map[string]*Message
	deliveries      map[string]*Delivery
	channels        map[string]*Channel
	webhooks        map[string]*Webhook
	aiConfig        AIConfig
	aiExecutions    []*AIExecution
	subscribers     map[int]chan RealtimeEvent
	subscriberSeq   int
}

func NewService() *Service {
	return &Service{
		apps:            make(map[string]*App),
		appIDsByService: make(map[string]string),
		customers:       make(map[string]*Customer),
		conversations:   make(map[string]*Conversation),
		messages:        make(map[string]*Message),
		deliveries:      make(map[string]*Delivery),
		channels:        make(map[string]*Channel),
		webhooks:        make(map[string]*Webhook),
		aiConfig: AIConfig{
			Prompt:              "You are a helpful customer service assistant.",
			HistoryMessageCount: 20,
			AutoReplyEnabled:    true,
			ReportingEnabled:    true,
		},
		subscribers: make(map[int]chan RealtimeEvent),
	}
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	clone := make([]string, len(values))
	copy(clone, values)
	return clone
}

func cloneMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	clone := make(map[string]any, len(values))
	for key, value := range values {
		clone[key] = value
	}
	return clone
}

func (s *Service) RegisterApp(_ context.Context, req RegisterAppRequest) (App, error) {
	if req.ServiceKey == "" {
		return App{}, badRequest("serviceKey is required")
	}
	if req.DisplayName == "" {
		return App{}, badRequest("displayName is required")
	}

	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()

	if appID, ok := s.appIDsByService[req.ServiceKey]; ok {
		app := s.apps[appID]
		app.DisplayName = req.DisplayName
		app.Kind = req.Kind
		app.Version = req.Version
		app.Capabilities = cloneStrings(req.Capabilities)
		app.UpdatedAt = now
		return *app, nil
	}

	id := "app_" + idutil.GenShortId()
	app := &App{
		ID:              id,
		ServiceKey:      req.ServiceKey,
		DisplayName:     req.DisplayName,
		Kind:            req.Kind,
		Version:         req.Version,
		Capabilities:    cloneStrings(req.Capabilities),
		Status:          "active",
		AccessToken:     "at_" + idutil.GenId(18),
		LeaseTTLSeconds: 60,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if req.LeaseTTLSeconds > 0 {
		app.LeaseTTLSeconds = req.LeaseTTLSeconds
	}
	s.apps[id] = app
	s.appIDsByService[req.ServiceKey] = id
	result := *app
	s.publishLocked("app.registered", id, result)
	return result, nil
}

func (s *Service) ListApps(_ context.Context) []App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	apps := make([]App, 0, len(s.apps))
	for _, app := range s.apps {
		apps = append(apps, *app)
	}
	sort.Slice(apps, func(i, j int) bool { return apps[i].CreatedAt.Before(apps[j].CreatedAt) })
	return apps
}

func (s *Service) GetApp(_ context.Context, id string) (App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, ok := s.apps[id]
	if !ok {
		return App{}, notFound("app %s not found", id)
	}
	return *app, nil
}

func (s *Service) Heartbeat(_ context.Context, req HeartbeatRequest) (App, error) {
	if req.AppID == "" {
		return App{}, badRequest("appId is required")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	app, ok := s.apps[req.AppID]
	if !ok {
		return App{}, notFound("app %s not found", req.AppID)
	}
	ttl := app.LeaseTTLSeconds
	if req.TTLSeconds > 0 {
		ttl = req.TTLSeconds
		app.LeaseTTLSeconds = ttl
	}
	if req.Version != "" {
		app.Version = req.Version
	}
	if len(req.Capabilities) > 0 {
		app.Capabilities = cloneStrings(req.Capabilities)
	}
	app.LastHeartbeatAt = now
	app.LeaseExpiresAt = now.Add(time.Duration(ttl) * time.Second)
	app.UpdatedAt = now
	result := *app
	s.publishLocked("app.heartbeat", app.ID, result)
	return result, nil
}

func (s *Service) SetAppStatus(_ context.Context, id string, enabled bool) (App, error) {
	now := time.Now().UTC()
	status := "disabled"
	if enabled {
		status = "active"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	app, ok := s.apps[id]
	if !ok {
		return App{}, notFound("app %s not found", id)
	}
	app.Status = status
	app.UpdatedAt = now
	result := *app
	s.publishLocked("app.status_changed", id, result)
	return result, nil
}

func (s *Service) CreateCustomer(_ context.Context, req CreateCustomerRequest) (Customer, error) {
	if req.Name == "" {
		return Customer{}, badRequest("name is required")
	}
	now := time.Now().UTC()
	customer := &Customer{
		ID:        "cus_" + idutil.GenShortId(),
		Name:      req.Name,
		Channel:   req.Channel,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if req.ExternalID != "" {
		identityType := req.IdentityType
		if identityType == "" {
			identityType = "external"
		}
		customer.Identities = append(customer.Identities, CustomerIdentity{
			Type:        identityType,
			ExternalID:  req.ExternalID,
			DisplayName: req.DisplayName,
			CreatedAt:   now,
		})
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.customers[customer.ID] = customer
	result := *customer
	s.publishLocked("customer.created", customer.ID, result)
	return result, nil
}

func (s *Service) GetCustomer(_ context.Context, id string) (Customer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	customer, ok := s.customers[id]
	if !ok {
		return Customer{}, notFound("customer %s not found", id)
	}
	return *customer, nil
}

func (s *Service) AddIdentity(_ context.Context, customerID string, req AddIdentityRequest) (Customer, error) {
	if req.Type == "" {
		return Customer{}, badRequest("type is required")
	}
	if req.ExternalID == "" {
		return Customer{}, badRequest("externalId is required")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	customer, ok := s.customers[customerID]
	if !ok {
		return Customer{}, notFound("customer %s not found", customerID)
	}
	for _, identity := range customer.Identities {
		if identity.Type == req.Type && identity.ExternalID == req.ExternalID {
			return *customer, nil
		}
	}
	customer.Identities = append(customer.Identities, CustomerIdentity{
		Type:        req.Type,
		ExternalID:  req.ExternalID,
		DisplayName: req.DisplayName,
		CreatedAt:   now,
	})
	customer.UpdatedAt = now
	result := *customer
	s.publishLocked("customer.identity_added", customer.ID, result)
	return result, nil
}

func (s *Service) MergeCustomer(_ context.Context, targetID string, req MergeCustomerRequest) (Customer, error) {
	if req.SourceCustomerID == "" {
		return Customer{}, badRequest("sourceCustomerId is required")
	}
	if req.SourceCustomerID == targetID {
		return Customer{}, badRequest("source and target customer are the same")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	target, ok := s.customers[targetID]
	if !ok {
		return Customer{}, notFound("customer %s not found", targetID)
	}
	source, ok := s.customers[req.SourceCustomerID]
	if !ok {
		return Customer{}, notFound("customer %s not found", req.SourceCustomerID)
	}
	target.Identities = append(target.Identities, source.Identities...)
	target.UpdatedAt = now
	delete(s.customers, source.ID)
	for _, conversation := range s.conversations {
		if conversation.CustomerID == source.ID {
			conversation.CustomerID = target.ID
			conversation.UpdatedAt = now
		}
	}
	result := *target
	s.publishLocked("customer.merged", target.ID, result)
	return result, nil
}
