package model

import (
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*AccountToken)(nil)

type AccountToken struct {
	base.BaseModel
	TokenKey   string     `gorm:"column:token_key;size:64;not null;uniqueIndex"`
	AccountKey string     `gorm:"column:account_key;size:64;not null;index"`
	Name       string     `gorm:"column:name;size:255"`
	TenantID   string     `gorm:"column:tenant_id;size:64;not null;index"`
	Kind       string     `gorm:"column:kind;size:32;not null;default:business;index"`
	ScopesJSON string     `gorm:"column:scopes;type:json"`
	TokenHash  string     `gorm:"column:token_hash;size:64;not null;uniqueIndex"`
	Device     string     `gorm:"column:device;size:255"`
	IP         string     `gorm:"column:ip;size:64"`
	Status     string     `gorm:"column:status;size:32;not null;default:active"`
	ExpiresAt  *time.Time `gorm:"column:expires_at;index"`
	RevokedAt  *time.Time `gorm:"column:revoked_at"`
	LastUsedAt *time.Time `gorm:"column:last_used_at"`
}

func (*AccountToken) TableName() string { return "account_tokens" }

func (t *AccountToken) GetID() field.IID   { return t.BaseModel.GetID() }
func (t *AccountToken) SetID(id field.IID) { t.BaseModel.SetID(id) }
func (t *AccountToken) IDColumn() string   { return t.BaseModel.IDColumn() }
func (t *AccountToken) Create()            { t.BaseModel.Create() }
func (t *AccountToken) Update()            { t.BaseModel.Update() }
