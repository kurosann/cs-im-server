package model

import (
	"encoding/json"
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"

	"gorm.io/gorm"
)

var _ damodel.Table = (*Webhook)(nil)

type Webhook struct {
	base.BaseModel
	WebhookKey string   `gorm:"column:webhook_key;size:64;uniqueIndex" json:"webhookId"`
	Name       string   `gorm:"column:name;size:255;not null" json:"name"`
	URL        string   `gorm:"column:url;size:512;not null" json:"url"`
	EventsJSON string   `gorm:"column:events;type:json" json:"-"`
	Events     []string `gorm:"-" json:"events"`
	Secret     string   `gorm:"column:secret;size:255" json:"secret"`
	Status     string   `gorm:"column:status;size:32;not null;default:disabled" json:"status"`
}

func (*Webhook) TableName() string { return "webhooks" }

func (w *Webhook) GetID() field.IID   { return w.BaseModel.GetID() }
func (w *Webhook) SetID(id field.IID) { w.BaseModel.SetID(id) }
func (w *Webhook) IDColumn() string   { return w.BaseModel.IDColumn() }
func (w *Webhook) Create()            { w.BaseModel.Create() }
func (w *Webhook) Update()            { w.BaseModel.Update() }

func (w *Webhook) BeforeSave(*gorm.DB) error {
	data, _ := json.Marshal(w.Events)
	w.EventsJSON = string(data)
	return nil
}

func (w *Webhook) AfterFind(*gorm.DB) error {
	if w.EventsJSON != "" {
		_ = json.Unmarshal([]byte(w.EventsJSON), &w.Events)
	}
	return nil
}

func (w *Webhook) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		WebhookKey string   `json:"webhookId"`
		Name       string   `json:"name"`
		URL        string   `json:"url"`
		Events     []string `json:"events"`
		Secret     string   `json:"secret"`
		Status     string   `json:"status"`
		CreatedAt  string   `json:"createdAt"`
		UpdatedAt  string   `json:"updatedAt"`
	}{
		WebhookKey: w.WebhookKey,
		Name:       w.Name,
		URL:        w.URL,
		Events:     w.Events,
		Secret:     w.Secret,
		Status:     w.Status,
		CreatedAt:  w.BaseModel.CreateAt.Format(time.RFC3339Nano),
		UpdatedAt:  w.BaseModel.UpdateAt.Format(time.RFC3339Nano),
	})
}
