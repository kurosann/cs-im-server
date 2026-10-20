package dto

import (
	"cs-im-server/internal/module/gateway/internal/model"
	"cs-im-server/pkg/seqquery"
)

type CreateMessageRequest struct {
	ConversationKey string              `json:"conversationId"`
	CustomerID      string              `json:"customerId"`
	Sender          model.MessageSender `json:"sender"`
	Source          model.MessageSource `json:"source"`
	Type            string              `json:"type"`
	Text            string              `json:"text"`
	Content         *model.MessageMedia `json:"content"`
}

type UpdateMessageStatusRequest struct {
	Status string `json:"status"`
}

type MessageHistoryFilter = seqquery.Filter

type MessageHistoryPage struct {
	Items         []model.Message `json:"items"`
	NextAfterSeq  int64           `json:"nextAfterSeq"`
	NextBeforeSeq int64           `json:"nextBeforeSeq"`
	HasMore       bool            `json:"hasMore"`
}

type MessageRef struct {
	MessageKey string
}
