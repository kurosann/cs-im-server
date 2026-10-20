package dto

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

type EnsureConversationRequest struct {
	CustomerExternalID string `json:"customerExternalId"`
	CustomerName       string `json:"customerName"`
	Channel            string `json:"channel"`
	Subject            string `json:"subject"`
}
