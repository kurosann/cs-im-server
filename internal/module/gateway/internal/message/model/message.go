package model

import (
	"encoding/json"
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*Message)(nil)

type Message struct {
	base.BaseModel
	MessageKey       string     `gorm:"column:message_key;size:64;uniqueIndex" json:"messageId"`
	Seq              int64      `gorm:"column:seq;not null;default:0" json:"seq"`
	ConversationKey  string     `gorm:"column:conversation_key;size:64;not null;index:idx_msg_conv" json:"conversationId"`
	CustomerKey      string     `gorm:"column:customer_key;size:64;not null" json:"customerId"`
	SenderRole       string     `gorm:"column:sender_role;size:32;not null" json:"-"`
	SenderID         string     `gorm:"column:sender_id;size:64;not null" json:"-"`
	SourceAppKey     string     `gorm:"column:source_app_key;size:64" json:"-"`
	SourceExternalID string     `gorm:"column:source_external_id;size:255" json:"-"`
	SourceSentAt     *time.Time `gorm:"column:source_sent_at" json:"-"`
	SourceDedupKey   *string    `gorm:"column:source_dedup_key;size:384;uniqueIndex" json:"-"`
	Type             string     `gorm:"column:type;size:32;not null;default:text" json:"type"`
	Text             string     `gorm:"column:text;type:text" json:"text,omitempty"`
	Status           string     `gorm:"column:status;size:32;not null;default:received" json:"status"`
}

func (*Message) TableName() string { return "messages" }

func (m *Message) GetID() field.IID   { return m.BaseModel.GetID() }
func (m *Message) SetID(id field.IID) { m.BaseModel.SetID(id) }
func (m *Message) IDColumn() string   { return m.BaseModel.IDColumn() }
func (m *Message) Create()            { m.BaseModel.Create() }
func (m *Message) Update()            { m.BaseModel.Update() }

func (m Message) MarshalJSON() ([]byte, error) {
	var source *MessageSource
	if m.SourceAppKey != "" || m.SourceExternalID != "" {
		source = &MessageSource{AppID: m.SourceAppKey, ExternalID: m.SourceExternalID, SentAt: m.SourceSentAt}
	}
	return json.Marshal(struct {
		MessageKey      string         `json:"messageId"`
		Seq             int64          `json:"seq"`
		ConversationKey string         `json:"conversationId"`
		CustomerKey     string         `json:"customerId"`
		Sender          MessageSender  `json:"sender"`
		Source          *MessageSource `json:"source,omitempty"`
		Type            string         `json:"type"`
		Text            string         `json:"text,omitempty"`
		Status          string         `json:"status"`
		ReceivedAt      string         `json:"receivedAt"`
		CreatedAt       string         `json:"createdAt"`
		UpdatedAt       string         `json:"updatedAt"`
	}{
		MessageKey:      m.MessageKey,
		Seq:             m.Seq,
		ConversationKey: m.ConversationKey,
		CustomerKey:     m.CustomerKey,
		Sender:          MessageSender{Role: m.SenderRole, ID: m.SenderID},
		Source:          source,
		Type:            m.Type,
		Text:            m.Text,
		Status:          m.Status,
		ReceivedAt:      m.BaseModel.CreateAt.Format(time.RFC3339Nano),
		CreatedAt:       m.CreatedAt().Format(time.RFC3339Nano),
		UpdatedAt:       m.BaseModel.UpdateAt.Format(time.RFC3339Nano),
	})
}

func (m *Message) CreatedAt() time.Time {
	if m.SourceSentAt != nil {
		return *m.SourceSentAt
	}
	return m.BaseModel.CreateAt
}

func (m *Message) UnmarshalJSON(data []byte) error {
	var v struct {
		MessageKey      string         `json:"messageId"`
		Seq             int64          `json:"seq"`
		ConversationKey string         `json:"conversationId"`
		CustomerKey     string         `json:"customerId"`
		Sender          MessageSender  `json:"sender"`
		Source          *MessageSource `json:"source,omitempty"`
		Type            string         `json:"type"`
		Text            string         `json:"text,omitempty"`
		Status          string         `json:"status"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	m.MessageKey = v.MessageKey
	m.Seq = v.Seq
	m.ConversationKey = v.ConversationKey
	m.CustomerKey = v.CustomerKey
	m.SenderRole = v.Sender.Role
	m.SenderID = v.Sender.ID
	if v.Source != nil {
		m.SourceAppKey = v.Source.AppID
		m.SourceExternalID = v.Source.ExternalID
		m.SourceSentAt = v.Source.SentAt
	}
	m.Type = v.Type
	m.Text = v.Text
	m.Status = v.Status
	return nil
}

type MessageSender struct {
	Role string `json:"role"`
	ID   string `json:"id"`
}

type MessageSource struct {
	AppID      string     `json:"appId,omitempty"`
	ExternalID string     `json:"externalId,omitempty"`
	SentAt     *time.Time `json:"sentAt,omitempty"`
}
