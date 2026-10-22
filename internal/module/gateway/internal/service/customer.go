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
	loadOnce      sync.Once
	loadErr       error
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
	if err := s.ensureLoaded(); err != nil {
		return Customer{}, err
	}
	if req.ExternalID == "" {
		return Customer{}, badRequest("customerExternalId is required")
	}
	identityType := "app:" + appID
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, customer := range s.customers {
		for _, identity := range customer.Identities {
			if identity.Type == identityType && identity.ExternalID == req.ExternalID {
				if req.DisplayName != "" {
					if customer.Name != req.DisplayName {
						customer.Name = req.DisplayName
					}
					for index := range customer.Identities {
						if customer.Identities[index].Type == identityType && customer.Identities[index].ExternalID == req.ExternalID {
							customer.Identities[index].DisplayName = req.DisplayName
						}
					}
					if err := s.accessor.Save(context.Background(), customer); err != nil {
						return Customer{}, err
					}
				}
				return *customer, nil
			}
		}
	}

	now := time.Now().UTC()
	name := req.DisplayName
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
	s.realtime.Publish("customer.created", customer.CustomerKey, customerToDTO(&result))
	return result, nil
}

func (s *CustomerService) Create(_ context.Context, req gatewaydto.CreateCustomerRequest) (Customer, error) {
	if err := s.ensureLoaded(); err != nil {
		return Customer{}, err
	}
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
	s.realtime.Publish("customer.created", customer.CustomerKey, customerToDTO(&result))
	return result, nil
}

func (s *CustomerService) Get(_ context.Context, id string) (Customer, error) {
	if err := s.ensureLoaded(); err != nil {
		return Customer{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	customer, ok := s.customers[id]
	if !ok {
		return Customer{}, notFound("customer %s not found", id)
	}
	return *customer, nil
}

func (s *CustomerService) AddIdentity(_ context.Context, customerID string, req gatewaydto.AddIdentityRequest) (Customer, error) {
	if err := s.ensureLoaded(); err != nil {
		return Customer{}, err
	}
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
	s.realtime.Publish("customer.identity_added", customer.CustomerKey, customerToDTO(&result))
	return result, nil
}

func (s *CustomerService) Merge(_ context.Context, targetID string, req gatewaydto.MergeCustomerRequest) (Customer, error) {
	if err := s.ensureLoaded(); err != nil {
		return Customer{}, err
	}
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
	s.realtime.Publish("customer.merged", target.CustomerKey, customerToDTO(target))
	return result, nil
}

func (s *CustomerService) Load() error {
	return s.ensureLoaded()
}

func (s *CustomerService) ensureLoaded() error {
	s.loadOnce.Do(func() {
		s.loadErr = s.load()
	})
	return s.loadErr
}

func (s *CustomerService) load() error {
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
