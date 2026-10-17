package model

import (
	"encoding/json"
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"

	"gorm.io/gorm"
)

var _ damodel.Table = (*Channel)(nil)

type Channel struct {
	base.BaseModel
	ChannelKey   string         `gorm:"column:channel_key;size:64;uniqueIndex" json:"channelId"`
	Name         string         `gorm:"column:name;size:255;not null" json:"name"`
	Type         string         `gorm:"column:type;size:64;not null" json:"type"`
	Endpoint     string         `gorm:"column:endpoint;size:512" json:"endpoint,omitempty"`
	ConfigJSON   string         `gorm:"column:config;type:json" json:"-"`
	Config       map[string]any `gorm:"-" json:"config,omitempty"`
	Status       string         `gorm:"column:status;size:32;not null;default:active" json:"status"`
	LastTestedAt *time.Time     `gorm:"column:last_tested_at" json:"lastTestedAt"`
}

func (*Channel) TableName() string { return "channels" }

func (c *Channel) GetID() field.IID   { return c.BaseModel.GetID() }
func (c *Channel) SetID(id field.IID) { c.BaseModel.SetID(id) }
func (c *Channel) IDColumn() string   { return c.BaseModel.IDColumn() }
func (c *Channel) Create()            { c.BaseModel.Create() }
func (c *Channel) Update()            { c.BaseModel.Update() }

func (c *Channel) BeforeSave(*gorm.DB) error {
	data, _ := json.Marshal(c.Config)
	c.ConfigJSON = string(data)
	return nil
}

func (c *Channel) AfterFind(*gorm.DB) error {
	if c.ConfigJSON != "" {
		_ = json.Unmarshal([]byte(c.ConfigJSON), &c.Config)
	}
	return nil
}

func (c *Channel) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ChannelKey   string         `json:"channelId"`
		Name         string         `json:"name"`
		Type         string         `json:"type"`
		Endpoint     string         `json:"endpoint,omitempty"`
		Config       map[string]any `json:"config,omitempty"`
		Status       string         `json:"status"`
		LastTestedAt *time.Time     `json:"lastTestedAt"`
		CreatedAt    string         `json:"createdAt"`
		UpdatedAt    string         `json:"updatedAt"`
	}{
		ChannelKey:   c.ChannelKey,
		Name:         c.Name,
		Type:         c.Type,
		Endpoint:     c.Endpoint,
		Config:       c.Config,
		Status:       c.Status,
		LastTestedAt: c.LastTestedAt,
		CreatedAt:    c.BaseModel.CreateAt.Format(time.RFC3339Nano),
		UpdatedAt:    c.BaseModel.UpdateAt.Format(time.RFC3339Nano),
	})
}
