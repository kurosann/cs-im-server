package model

import (
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*ChannelCredential)(nil)

type ChannelCredential struct {
	base.BaseModel
	AppKey     string     `gorm:"column:app_key;size:64;not null;uniqueIndex"`
	TenantID   string     `gorm:"column:tenant_id;size:64;not null;index"`
	Name       string     `gorm:"column:name;size:255;not null"`
	SecretHash string     `gorm:"column:secret_hash;size:64;not null;uniqueIndex"`
	Status     string     `gorm:"column:status;size:32;not null;default:active"`
	ExpiresAt  *time.Time `gorm:"column:expires_at;index"`
	RevokedAt  *time.Time `gorm:"column:revoked_at"`
	LastUsedAt *time.Time `gorm:"column:last_used_at"`
}

func (*ChannelCredential) TableName() string { return "channel_credentials" }

func (c *ChannelCredential) GetID() field.IID   { return c.BaseModel.GetID() }
func (c *ChannelCredential) SetID(id field.IID) { c.BaseModel.SetID(id) }
func (c *ChannelCredential) IDColumn() string   { return c.BaseModel.IDColumn() }
func (c *ChannelCredential) Create()            { c.BaseModel.Create() }
func (c *ChannelCredential) Update()            { c.BaseModel.Update() }
