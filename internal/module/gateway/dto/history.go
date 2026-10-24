package dto

import "time"

type HistoryRequest struct {
	RequestID          string               `json:"requestId"`
	AppID              string               `json:"appId"`
	ConversationID     string               `json:"conversationId"`
	CustomerExternalID string               `json:"customerExternalId"`
	Status             HistoryRequestStatus `json:"status"`
	CreatedAt          time.Time            `json:"createdAt"`
	CompletedAt        *time.Time           `json:"completedAt,omitempty"`
}

type CompleteHistoryRequestInput struct {
	RequestID string               `json:"requestId"`
	Status    HistoryRequestStatus `json:"status"`
}
