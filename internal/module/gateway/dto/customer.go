package dto

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

type EnsureExternalCustomerRequest struct {
	ExternalID  string `json:"externalId"`
	DisplayName string `json:"displayName"`
	Channel     string `json:"channel"`
}
