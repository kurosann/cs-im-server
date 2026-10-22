package dto

import "time"

type CreateCustomerRequest struct {
	Name         string `json:"name"`
	Channel      string `json:"channel"`
	IdentityType string `json:"identityType"`
	ExternalID   string `json:"externalId"`
	DisplayName  string `json:"displayName"`
}

type CustomerIdentity struct {
	Type        string    `json:"type"`
	ExternalID  string    `json:"externalId"`
	DisplayName string    `json:"displayName,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Customer struct {
	CustomerKey string             `json:"customerId"`
	Name        string             `json:"name"`
	Channel     string             `json:"channel,omitempty"`
	Status      string             `json:"status"`
	Identities  []CustomerIdentity `json:"identities,omitempty"`
	CreatedAt   string             `json:"createdAt"`
	UpdatedAt   string             `json:"updatedAt"`
}

type AddIdentityRequest struct {
	Type        string `json:"type"`
	ExternalID  string `json:"externalId"`
	DisplayName string `json:"displayName"`
}

type MergeCustomerRequest struct {
	SourceCustomerID string `json:"sourceCustomerId"`
}

type EnsureExternalCustomerRequest struct {
	ExternalID  string `json:"externalId"`
	DisplayName string `json:"displayName"`
	Channel     string `json:"channel"`
}
