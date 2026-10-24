package dto

import "time"

type CreateConversationRequest struct {
	AppID      string         `json:"appId"`
	CustomerID string         `json:"customerId"`
	Channel    string         `json:"channel"`
	Subject    string         `json:"subject"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type Conversation struct {
	ConversationKey    string         `json:"conversationId"`
	AppKey             string         `json:"appId"`
	CustomerKey        string         `json:"customerId"`
	CustomerExternalID string         `json:"customerExternalId,omitempty"`
	Channel            string         `json:"channel,omitempty"`
	Subject            string         `json:"subject,omitempty"`
	Status             string         `json:"status"`
	Assignee           string         `json:"assignee,omitempty"`
	ForwardedTo        string         `json:"forwardedTo,omitempty"`
	CloseReason        string         `json:"closeReason,omitempty"`
	ClosedAt           *time.Time     `json:"closedAt,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
	ReceivedAt         string         `json:"receivedAt"`
	CreatedAt          string         `json:"createdAt"`
	UpdatedAt          string         `json:"updatedAt"`
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
	CustomerExternalID string         `json:"customerExternalId"`
	CustomerName       string         `json:"customerName"`
	Channel            string         `json:"channel"`
	Subject            string         `json:"subject"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}
