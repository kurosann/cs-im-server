package dto

import "errors"

import "time"

type MessageSender struct {
	Role SenderRole `json:"role"`
	ID   string     `json:"id"`
}

type MessageSource struct {
	AppID      string     `json:"appId,omitempty"`
	ExternalID string     `json:"externalId,omitempty"`
	SentAt     *time.Time `json:"sentAt,omitempty"`
}

type MessageMedia struct {
	URL             string `json:"url"`
	MimeType        string `json:"mimeType,omitempty"`
	FileName        string `json:"fileName,omitempty"`
	Size            int64  `json:"size,omitempty"`
	Width           int    `json:"width,omitempty"`
	Height          int    `json:"height,omitempty"`
	DurationSeconds int    `json:"duration,omitempty"`
	ThumbnailURL    string `json:"thumbnailUrl,omitempty"`
	Checksum        string `json:"checksum,omitempty"`
}

type CreateMessageRequest struct {
	ConversationID string        `json:"conversationId"`
	CustomerID     string        `json:"customerId"`
	Sender         MessageSender `json:"sender"`
	Source         MessageSource `json:"source"`
	Type           MessageType   `json:"type"`
	Text           string        `json:"text"`
	Content        *MessageMedia `json:"content"`
	Status         MessageStatus `json:"status,omitempty"`
}

type BatchCreateMessagesRequest struct {
	Items []CreateMessageRequest `json:"items"`
}

type UpdateHistoryRequestStatusInput struct {
	RequestID string               `json:"requestId"`
	Status    HistoryRequestStatus `json:"status"`
}

type HistoryRequest struct {
	RequestID          string               `json:"requestId"`
	AppID              string               `json:"appId"`
	ConversationID     string               `json:"conversationId"`
	CustomerExternalID string               `json:"customerExternalId"`
	Status             HistoryRequestStatus `json:"status"`
	CreatedAt          time.Time            `json:"createdAt"`
	CompletedAt        *time.Time           `json:"completedAt,omitempty"`
}

func (req CreateMessageRequest) Validate() error {
	if !req.Sender.Role.Valid() {
		return errors.New("unsupported sender role")
	}
	if req.Status != "" && !req.Status.Valid() {
		return errors.New("unsupported message status")
	}
	if req.Type == "" {
		req.Type = MessageTypeText
	}
	if !req.Type.Valid() {
		return errors.New("unsupported message type")
	}
	return nil
}

func (req BatchCreateMessagesRequest) Validate() error {
	for _, item := range req.Items {
		if err := item.Validate(); err != nil {
			return err
		}
	}
	return nil
}
