package model

import (
	"encoding/json"
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"

	"gorm.io/gorm"
)

var _ damodel.Table = (*Customer)(nil)

type Customer struct {
	base.BaseModel
	CustomerKey    string             `gorm:"column:customer_key;size:64;uniqueIndex" json:"customerId"`
	Name           string             `gorm:"column:name;size:255;not null" json:"name"`
	Channel        string             `gorm:"column:channel;size:64" json:"channel,omitempty"`
	Status         string             `gorm:"column:status;size:32;not null;default:active" json:"status"`
	IdentitiesJSON string             `gorm:"column:identities;type:json" json:"-"`
	Identities     []CustomerIdentity `gorm:"-" json:"identities,omitempty"`
}

func (*Customer) TableName() string { return "customers" }

func (c *Customer) GetID() field.IID   { return c.BaseModel.GetID() }
func (c *Customer) SetID(id field.IID) { c.BaseModel.SetID(id) }
func (c *Customer) IDColumn() string   { return c.BaseModel.IDColumn() }
func (c *Customer) Create()            { c.BaseModel.Create() }
func (c *Customer) Update()            { c.BaseModel.Update() }

func (c *Customer) BeforeSave(*gorm.DB) error {
	data, _ := json.Marshal(c.Identities)
	c.IdentitiesJSON = string(data)
	return nil
}

func (c *Customer) AfterFind(*gorm.DB) error {
	if c.IdentitiesJSON != "" {
		_ = json.Unmarshal([]byte(c.IdentitiesJSON), &c.Identities)
	}
	return nil
}

func (c *Customer) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		CustomerKey string             `json:"customerId"`
		Name        string             `json:"name"`
		Channel     string             `json:"channel,omitempty"`
		Status      string             `json:"status"`
		Identities  []CustomerIdentity `json:"identities,omitempty"`
		CreatedAt   string             `json:"createdAt"`
		UpdatedAt   string             `json:"updatedAt"`
	}{
		CustomerKey: c.CustomerKey,
		Name:        c.Name,
		Channel:     c.Channel,
		Status:      c.Status,
		Identities:  c.Identities,
		CreatedAt:   c.BaseModel.CreateAt.Format(time.RFC3339Nano),
		UpdatedAt:   c.BaseModel.UpdateAt.Format(time.RFC3339Nano),
	})
}

func (c *Customer) UnmarshalJSON(data []byte) error {
	var v struct {
		CustomerKey string             `json:"customerId"`
		Name        string             `json:"name"`
		Channel     string             `json:"channel,omitempty"`
		Status      string             `json:"status"`
		Identities  []CustomerIdentity `json:"identities,omitempty"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	c.CustomerKey = v.CustomerKey
	c.Name = v.Name
	c.Channel = v.Channel
	c.Status = v.Status
	c.Identities = v.Identities
	return nil
}

type CustomerIdentity struct {
	Type        string    `json:"type"`
	ExternalID  string    `json:"externalId"`
	DisplayName string    `json:"displayName,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}
