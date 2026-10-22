package dto

import (
	"time"

	"cs-im-server/pkg/seqquery"
)

type MessageSender struct {
	Role string `json:"role"`
	ID   string `json:"id"`
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

type Message struct {
	MessageKey      string         `json:"messageId"`
	Seq             int64          `json:"seq"`
	ConversationKey string         `json:"conversationId"`
	CustomerKey     string         `json:"customerId"`
	Sender          MessageSender  `json:"sender"`
	Source          *MessageSource `json:"source,omitempty"`
	Type            string         `json:"type"`
	Text            string         `json:"text,omitempty"`
	Content         *MessageMedia  `json:"content,omitempty"`
	Status          string         `json:"status"`
	ReceivedAt      string         `json:"receivedAt"`
	CreatedAt       string         `json:"createdAt"`
	UpdatedAt       string         `json:"updatedAt"`
}

type CreateMessageRequest struct {
	ConversationKey string        `json:"conversationId"`
	CustomerID      string        `json:"customerId"`
	Sender          MessageSender `json:"sender"`
	Source          MessageSource `json:"source"`
	Type            string        `json:"type"`
	Text            string        `json:"text"`
	Content         *MessageMedia `json:"content"`
	Status          string        `json:"status,omitempty"`
}

type UpdateMessageStatusRequest struct {
	Status string `json:"status"`
}

type MessageHistoryFilter = seqquery.Filter

type MessageHistoryPage struct {
	Items         []Message `json:"items"`
	NextAfterSeq  int64     `json:"nextAfterSeq"`
	NextBeforeSeq int64     `json:"nextBeforeSeq"`
	HasMore       bool      `json:"hasMore"`
}

type MessageOffset struct {
	ConversationKey  string     `json:"conversationId"`
	LatestSeq        int64      `json:"latestSeq"`
	LatestMessageID  string     `json:"latestMessageId,omitempty"`
	LatestExternalID string     `json:"latestExternalId,omitempty"`
	LatestSentAt     *time.Time `json:"latestSentAt,omitempty"`
}

type BatchCreateMessagesRequest struct {
	Items []CreateMessageRequest `json:"items"`
}

type BatchCreateMessagesResult struct {
	Items []Message `json:"items"`
}

type MessageRef struct {
	MessageKey string
}
