package dto

import "time"

type RelayMessageSender struct {
	Role string `json:"role"`
	ID   string `json:"id"`
}

type RelayMessageSource struct {
	ExternalID string     `json:"externalId,omitempty"`
	SentAt     *time.Time `json:"sentAt,omitempty"`
}

type RelayMessageMedia struct {
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

type RelayMessage struct {
	MessageKey      string              `json:"messageId"`
	Seq             int64               `json:"seq"`
	ConversationKey string              `json:"conversationId"`
	CustomerKey     string              `json:"customerId"`
	Sender          RelayMessageSender  `json:"sender"`
	Source          *RelayMessageSource `json:"source,omitempty"`
	Type            string              `json:"type"`
	Text            string              `json:"text,omitempty"`
	Content         *RelayMessageMedia  `json:"content,omitempty"`
	Status          string              `json:"status"`
	ReceivedAt      string              `json:"receivedAt"`
	CreatedAt       string              `json:"createdAt"`
	UpdatedAt       string              `json:"updatedAt"`
}

type CreateRelayMessageRequest struct {
	ConversationKey string             `json:"conversationId"`
	Type            string             `json:"type"`
	Text            string             `json:"text"`
	Content         *RelayMessageMedia `json:"content"`
}

type RelayMessageHistoryPage struct {
	Items         []RelayMessage `json:"items"`
	NextAfterSeq  int64          `json:"nextAfterSeq"`
	NextBeforeSeq int64          `json:"nextBeforeSeq"`
	HasMore       bool           `json:"hasMore"`
}

type RelayTranslateItem struct {
	Text       string `json:"text"`
	TargetLang string `json:"targetLang"`
	SourceLang string `json:"sourceLang"`
	ProviderID string `json:"providerId"`
}

type RelayTranslateRequest struct {
	Items []RelayTranslateItem `json:"items"`
}

type RelayTranslateResult struct {
	Text           string `json:"text"`
	TargetLang     string `json:"targetLang"`
	SourceLang     string `json:"sourceLang"`
	TranslatedText string `json:"translatedText"`
	Cached         bool   `json:"cached"`
	QualityTier    string `json:"qualityTier"`
}
