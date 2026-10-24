package model

import (
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*HistoryRequest)(nil)

type HistoryRequest struct {
	base.BaseModel
	RequestKey         string     `gorm:"column:request_key;size:64;uniqueIndex" json:"requestId"`
	AppKey             string     `gorm:"column:app_key;size:64;not null;index" json:"appId"`
	ConversationKey    string     `gorm:"column:conversation_key;size:64;not null;index" json:"conversationId"`
	CustomerExternalID string     `gorm:"column:customer_external_id;size:255" json:"customerExternalId"`
	Status             string     `gorm:"column:status;size:32;not null;default:pending;index" json:"status"`
	CreatedAt          time.Time  `gorm:"column:created_at" json:"createdAt"`
	CompletedAt        *time.Time `gorm:"column:completed_at" json:"completedAt,omitempty"`
}

func (*HistoryRequest) TableName() string { return "history_requests" }

func (r *HistoryRequest) GetID() field.IID   { return r.BaseModel.GetID() }
func (r *HistoryRequest) SetID(id field.IID) { r.BaseModel.SetID(id) }
func (r *HistoryRequest) IDColumn() string   { return r.BaseModel.IDColumn() }
func (r *HistoryRequest) Create()            { r.BaseModel.Create() }
func (r *HistoryRequest) Update()            { r.BaseModel.Update() }
