package model

import (
	"encoding/json"
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*Conversation)(nil)

type Conversation struct {
	base.BaseModel
	ConversationKey string     `gorm:"column:conversation_key;size:64;uniqueIndex" json:"conversationId"`
	AppKey          string     `gorm:"column:app_key;size:64;not null;index" json:"appId"`
	CustomerKey     string     `gorm:"column:customer_key;size:64;not null;index" json:"customerId"`
	Channel         string     `gorm:"column:channel;size:64" json:"channel,omitempty"`
	Subject         string     `gorm:"column:subject;size:255" json:"subject,omitempty"`
	Status          string     `gorm:"column:status;size:32;not null;default:open;index" json:"status"`
	Assignee        string     `gorm:"column:assignee;size:64" json:"assignee,omitempty"`
	ForwardedTo     string     `gorm:"column:forwarded_to;size:64" json:"forwardedTo,omitempty"`
	CloseReason     string     `gorm:"column:close_reason;size:255" json:"closeReason,omitempty"`
	ClosedAt        *time.Time `gorm:"column:closed_at" json:"closedAt,omitempty"`
}

func (*Conversation) TableName() string { return "conversations" }

func (c *Conversation) GetID() field.IID   { return c.BaseModel.GetID() }
func (c *Conversation) SetID(id field.IID) { c.BaseModel.SetID(id) }
func (c *Conversation) IDColumn() string   { return c.BaseModel.IDColumn() }
func (c *Conversation) Create()            { c.BaseModel.Create() }
func (c *Conversation) Update()            { c.BaseModel.Update() }

func (c *Conversation) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ConversationKey string     `json:"conversationId"`
		AppKey          string     `json:"appId"`
		CustomerKey     string     `json:"customerId"`
		Channel         string     `json:"channel,omitempty"`
		Subject         string     `json:"subject,omitempty"`
		Status          string     `json:"status"`
		Assignee        string     `json:"assignee,omitempty"`
		ForwardedTo     string     `json:"forwardedTo,omitempty"`
		CloseReason     string     `json:"closeReason,omitempty"`
		CreatedAt       string     `json:"createdAt"`
		UpdatedAt       string     `json:"updatedAt"`
		ClosedAt        *time.Time `json:"closedAt,omitempty"`
	}{
		ConversationKey: c.ConversationKey,
		AppKey:          c.AppKey,
		CustomerKey:     c.CustomerKey,
		Channel:         c.Channel,
		Subject:         c.Subject,
		Status:          c.Status,
		Assignee:        c.Assignee,
		ForwardedTo:     c.ForwardedTo,
		CloseReason:     c.CloseReason,
		CreatedAt:       c.BaseModel.CreateAt.Format(time.RFC3339Nano),
		UpdatedAt:       c.BaseModel.UpdateAt.Format(time.RFC3339Nano),
		ClosedAt:        c.ClosedAt,
	})
}
