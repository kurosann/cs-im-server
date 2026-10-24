package service

import (
	"context"
	"strings"
	"time"

	"vane-common/pkg/dataaccess"
	"vane-common/pkg/idutil"

	gatewaydto "cs-im-server/internal/module/gateway/dto"
	"cs-im-server/internal/module/gateway/internal/model"
)

type CustomerService struct {
	accessor dataaccess.DataAccessor[*model.Customer]
	realtime *RealtimeService
}

func NewCustomerService(realtime *RealtimeService) *CustomerService {
	return &CustomerService{
		realtime: realtime,
	}
}

func (s *CustomerService) EnsureExternal(_ context.Context, appID string, req gatewaydto.EnsureExternalCustomerRequest) (model.Customer, error) {
	if req.ExternalID == "" {
		return model.Customer{}, badRequest("customerExternalId is required")
	}
	identityType := "app:" + appID
	found, err := s.FindCustomerIdentities(context.Background(), identityType, req.ExternalID)
	if err != nil {
		return model.Customer{}, err
	}
	if found != nil {
		customer := found
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
						return model.Customer{}, err
					}
				}
				return *customer, nil
			}
		}
	}
	if isWhatsAppChannel(req.Channel) {
		candidates, customerErr := s.accessor.List(context.Background())
		if customerErr != nil {
			return model.Customer{}, customerErr
		}
		for _, candidate := range candidates {
			if customerMatchesWhatsAppIdentity(candidate, identityType, req.ExternalID) {
				return *candidate, nil
			}
		}
	}

	now := time.Now().UTC()
	name := req.DisplayName
	customer := &model.Customer{
		CustomerKey: "cus_" + idutil.GenShortId(),
		Name:        name,
		Channel:     req.Channel,
		Status:      "active",
		Identities: []model.CustomerIdentity{{
			Type:        identityType,
			ExternalID:  req.ExternalID,
			DisplayName: req.DisplayName,
			CreatedAt:   now,
		}},
	}
	if err := s.accessor.Save(context.Background(), customer); err != nil {
		return model.Customer{}, err
	}
	result := *customer
	s.realtime.Publish("customer.created", customer.CustomerKey, customerToDTO(&result))
	return result, nil
}

func isWhatsAppChannel(channel string) bool {
	channel = strings.ToLower(strings.TrimSpace(channel))
	return channel == "wa" || channel == "whatsapp"
}

func customerMatchesWhatsAppIdentity(customer *model.Customer, identityType, externalID string) bool {
	if customer == nil || identityType == "" || externalID == "" {
		return false
	}
	requestedPhone := canonicalCustomerExternalID("whatsapp", externalID)
	if requestedPhone == "" || requestedPhone == externalID {
		return false
	}
	for _, identity := range customer.Identities {
		if identity.Type != identityType {
			continue
		}
		if canonicalCustomerExternalID("whatsapp", identity.ExternalID) == requestedPhone {
			return true
		}
	}
	return false
}

func (s *CustomerService) Create(_ context.Context, req gatewaydto.CreateCustomerRequest) (model.Customer, error) {
	if req.Name == "" {
		return model.Customer{}, badRequest("name is required")
	}
	now := time.Now().UTC()
	customer := &model.Customer{
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
		customer.Identities = append(customer.Identities, model.CustomerIdentity{Type: identityType, ExternalID: req.ExternalID, DisplayName: req.DisplayName, CreatedAt: now})
	}
	if err := s.accessor.Save(context.Background(), customer); err != nil {
		return model.Customer{}, err
	}
	result := *customer
	s.realtime.Publish("customer.created", customer.CustomerKey, customerToDTO(&result))
	return result, nil
}

func (s *CustomerService) Get(_ context.Context, id string) (model.Customer, error) {
	customer, err := s.getByKey(context.Background(), id)
	if err != nil {
		return model.Customer{}, err
	}
	return *customer, nil
}

func (s *CustomerService) ExternalIDForCustomer(_ context.Context, id string) (string, error) {
	customer, err := s.getByKey(context.Background(), id)
	if err != nil {
		return "", notFound("customer %s not found", id)
	}
	for _, identity := range customer.Identities {
		if identity.Type == "external" {
			return identity.ExternalID, nil
		}
	}
	for _, identity := range customer.Identities {
		if identity.ExternalID != "" {
			return identity.ExternalID, nil
		}
	}
	return "", badRequest("customer %s has no external identity", id)
}

func (s *CustomerService) AddIdentity(_ context.Context, customerID string, req gatewaydto.AddIdentityRequest) (model.Customer, error) {
	if req.Type == "" {
		return model.Customer{}, badRequest("type is required")
	}
	if req.ExternalID == "" {
		return model.Customer{}, badRequest("externalId is required")
	}
	now := time.Now().UTC()
	customer, err := s.getByKey(context.Background(), customerID)
	if err != nil {
		return model.Customer{}, notFound("customer %s not found", customerID)
	}
	for _, identity := range customer.Identities {
		if identity.Type == req.Type && identity.ExternalID == req.ExternalID {
			return *customer, nil
		}
	}
	customer.Identities = append(customer.Identities, model.CustomerIdentity{Type: req.Type, ExternalID: req.ExternalID, DisplayName: req.DisplayName, CreatedAt: now})
	customer.UpdateAt = now
	if err := s.accessor.Save(context.Background(), customer); err != nil {
		return model.Customer{}, err
	}
	result := *customer
	s.realtime.Publish("customer.identity_added", customer.CustomerKey, customerToDTO(&result))
	return result, nil
}

func (s *CustomerService) Merge(_ context.Context, targetID string, req gatewaydto.MergeCustomerRequest) (model.Customer, error) {
	if req.SourceCustomerID == "" {
		return model.Customer{}, badRequest("sourceCustomerId is required")
	}
	if req.SourceCustomerID == targetID {
		return model.Customer{}, badRequest("source and target customer are the same")
	}
	target, targetErr := s.getByKey(context.Background(), targetID)
	source, sourceErr := s.getByKey(context.Background(), req.SourceCustomerID)
	if targetErr != nil || sourceErr != nil {
		if targetErr != nil {
			return model.Customer{}, notFound("customer %s not found", targetID)
		}
		return model.Customer{}, notFound("customer %s not found", req.SourceCustomerID)
	}
	if target == nil || source == nil {
		if target == nil {
			return model.Customer{}, notFound("customer %s not found", targetID)
		}
		return model.Customer{}, notFound("customer %s not found", req.SourceCustomerID)
	}
	if req.SourceCustomerID == targetID {
		return model.Customer{}, badRequest("source and target customer are the same")
	}
	now := time.Now().UTC()
	target.Identities = append(target.Identities, source.Identities...)
	target.UpdateAt = now
	if err := s.accessor.Save(context.Background(), target); err != nil {
		return model.Customer{}, err
	}
	if err := s.accessor.Delete(context.Background(), source); err != nil {
		return model.Customer{}, err
	}
	result := *target
	s.realtime.Publish("customer.merged", target.CustomerKey, customerToDTO(target))
	return result, nil
}

func (s *CustomerService) FindCustomerIdentities(ctx context.Context, identityType, externalID string) (*model.Customer, error) {
	items, err := s.accessor.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		for _, identity := range item.Identities {
			if identity.Type == identityType && identity.ExternalID == externalID {
				return item, nil
			}
		}
	}
	return nil, nil
}

func (s *CustomerService) getByKey(ctx context.Context, id string) (*model.Customer, error) {
	conditions := []dataaccess.QueryCondition{
		dataaccess.Eq("customer_key", id),
		dataaccess.Limit(1),
	}
	var found []*model.Customer
	if err := s.accessor.ListWithOption(ctx, &found, dataaccess.WithConditions(conditions...)); err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, notFound("customer %s not found", id)
	}
	return found[0], nil
}
