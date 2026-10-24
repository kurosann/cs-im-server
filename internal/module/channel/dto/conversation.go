package dto

type EnsureConversationRequest struct {
	CustomerExternalID string         `json:"customerExternalId"`
	CustomerName       string         `json:"customerName"`
	Channel            string         `json:"channel"`
	Subject            string         `json:"subject"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type HeartbeatRequest struct {
	AppID string `json:"appId"`
}
