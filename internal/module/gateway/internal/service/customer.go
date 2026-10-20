package service

import (
	"context"
	"sync"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
)

type CustomerService struct {
	mu            sync.RWMutex
	accessor      dataaccess.DataAccessor[*Customer]
	customers     map[string]*Customer
	realtime      *RealtimeService
	conversations ConversationRetargeter
}

func NewCustomerService(realtime *RealtimeService) *CustomerService {
	return &CustomerService{
		customers: make(map[string]*Customer),
		realtime:  realtime,
	}
}

func (s *CustomerService) SetConversations(conversations ConversationRetargeter) {
	s.conversations = conversations
}

func (s *CustomerService) EnsureExternal(_ context.Context, appID string, req gatewaydto.EnsureExternalCustomerRequest) (Customer, error) {
	if req.ExternalID == "" {
		return Customer{}, badRequest("customerExternalId is required")
	}
	identityType := "app:" + appID
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, customer := range s.customers {
		for _, identity := range customer.Identities {
			if identity.Type == identityType && identity.ExternalID == req.ExternalID {
				return *customer, nil
			}
		}
	}

	now := time.Now().UTC()
	name := req.DisplayName
	if name == "" {
		name = req.Channel + ":" + req.ExternalID
	}
	customer := &Customer{
		CustomerKey: "cus_" + idutil.GenShortId(),
		Name:        name,
		Channel:     req.Channel,
		Status:      "active",
		Identities: []CustomerIdentity{{
			Type:        identityType,
			ExternalID:  req.ExternalID,
			DisplayName: req.DisplayName,
			CreatedAt:   now,
		}},
	}
	if err := s.accessor.Save(context.Background(), customer); err != nil {
		return Customer{}, err
	}
	s.customers[customer.CustomerKey] = customer
	result := *customer
	s.realtime.Publish("customer.created", customer.CustomerKey, result)
	return result, nil
}

func (s *CustomerService) Create(_ context.Context, req gatewaydto.CreateCustomerRequest) (Customer, error) {
	if req.Name == "" {
		return Customer{}, badRequest("name is required")
	}
	now := time.Now().UTC()
	customer := &Customer{
		CustomerKey: "cus_" + idutil.GenShortId(),
		Name:        req.Name,
		Channel:     req.Channel,
		Status:      "active",
	}
	if req.ExternalID != "" {
		identityType := req.IdentityType
		if identityType == "" {
			identityType = "external"
		}
		customer.Identities = append(customer.Identities, CustomerIdentity{Type: identityType, ExternalID: req.ExternalID, DisplayName: req.DisplayName, CreatedAt: now})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.accessor.Save(context.Background(), customer); err != nil {
		return Customer{}, err
	}
	s.customers[customer.CustomerKey] = customer
	result := *customer
	s.realtime.Publish("customer.created", customer.CustomerKey, result)
	return result, nil
}

func (s *CustomerService) Get(_ context.Context, id string) (Customer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	customer, ok := s.customers[id]
	if !ok {
		return Customer{}, notFound("customer %s not found", id)
	}
	return *customer, nil
}

func (s *CustomerService) AddIdentity(_ context.Context, customerID string, req gatewaydto.AddIdentityRequest) (Customer, error) {
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
	customer.Identities = append(customer.Identities, CustomerIdentity{Type: req.Type, ExternalID: req.ExternalID, DisplayName: req.DisplayName, CreatedAt: now})
	customer.UpdateAt = now
	if err := s.accessor.Save(context.Background(), customer); err != nil {
		return Customer{}, err
	}
	result := *customer
	s.realtime.Publish("customer.identity_added", customer.CustomerKey, result)
	return result, nil
}

func (s *CustomerService) Merge(_ context.Context, targetID string, req gatewaydto.MergeCustomerRequest) (Customer, error) {
	if req.SourceCustomerID == "" {
		return Customer{}, badRequest("sourceCustomerId is required")
	}
	if req.SourceCustomerID == targetID {
		return Customer{}, badRequest("source and target customer are the same")
	}
	now := time.Now().UTC()
	s.mu.Lock()
	target, ok := s.customers[targetID]
	source, sourceExists := s.customers[req.SourceCustomerID]
	if !ok || !sourceExists {
		s.mu.Unlock()
		if !ok {
			return Customer{}, notFound("customer %s not found", targetID)
		}
		return Customer{}, notFound("customer %s not found", req.SourceCustomerID)
	}
	target.Identities = append(target.Identities, source.Identities...)
	target.UpdateAt = now
	if err := s.accessor.Save(context.Background(), target); err != nil {
		s.mu.Unlock()
		return Customer{}, err
	}
	delete(s.customers, source.CustomerKey)
	s.mu.Unlock()
	if s.conversations != nil {
		if err := s.conversations.RetargetCustomer(source.CustomerKey, target.CustomerKey); err != nil {
			return Customer{}, err
		}
	}
	result := *target
	s.realtime.Publish("customer.merged", target.CustomerKey, result)
	return result, nil
}

func (s *CustomerService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.accessor.List(context.Background())
	if err != nil {
		return err
	}
	for _, item := range items {
		s.customers[item.CustomerKey] = item
	}
	return nil
}
