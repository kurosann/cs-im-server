package model

import (
	"encoding/json"
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*Delivery)(nil)

type Delivery struct {
	base.BaseModel
	DeliveryKey     string `gorm:"column:delivery_key;size:64;uniqueIndex" json:"deliveryId"`
	ConversationKey string `gorm:"column:conversation_key;size:64;not null;index" json:"conversationId"`
	MessageKey      string `gorm:"column:message_key;size:64;not null" json:"messageId"`
	ChannelKey      string `gorm:"column:channel_key;size:64;not null" json:"channelId"`
	Status          string `gorm:"column:status;size:32;not null;default:queued" json:"status"`
}

func (*Delivery) TableName() string { return "deliveries" }

func (d *Delivery) GetID() field.IID   { return d.BaseModel.GetID() }
func (d *Delivery) SetID(id field.IID) { d.BaseModel.SetID(id) }
func (d *Delivery) IDColumn() string   { return d.BaseModel.IDColumn() }
func (d *Delivery) Create()            { d.BaseModel.Create() }
func (d *Delivery) Update()            { d.BaseModel.Update() }

func (d *Delivery) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		DeliveryKey     string `json:"deliveryId"`
		ConversationKey string `json:"conversationId"`
		MessageKey      string `json:"messageId"`
		ChannelKey      string `json:"channelId"`
		Status          string `json:"status"`
		CreatedAt       string `json:"createdAt"`
		UpdatedAt       string `json:"updatedAt"`
	}{
		DeliveryKey:     d.DeliveryKey,
		ConversationKey: d.ConversationKey,
		MessageKey:      d.MessageKey,
		ChannelKey:      d.ChannelKey,
		Status:          d.Status,
		CreatedAt:       d.BaseModel.CreateAt.Format(time.RFC3339Nano),
		UpdatedAt:       d.BaseModel.UpdateAt.Format(time.RFC3339Nano),
	})
}
