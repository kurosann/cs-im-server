package model

import (
	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*AccountToken)(nil)

type AccountToken struct {
	base.BaseModel
	AccountKey string `gorm:"column:account_key;size:64;not null;uniqueIndex:idx_account_tokens_account"`
	Name       string `gorm:"column:name;size:255"`
	TenantID   string `gorm:"column:tenant_id;size:64;not null;index"`
	TokenHash  string `gorm:"column:token_hash;size:64;not null;uniqueIndex"`
	Status     string `gorm:"column:status;size:32;not null;default:active"`
}

func (*AccountToken) TableName() string { return "account_tokens" }

func (t *AccountToken) GetID() field.IID   { return t.BaseModel.GetID() }
func (t *AccountToken) SetID(id field.IID) { t.BaseModel.SetID(id) }
func (t *AccountToken) IDColumn() string   { return t.BaseModel.IDColumn() }
func (t *AccountToken) Create()            { t.BaseModel.Create() }
func (t *AccountToken) Update()            { t.BaseModel.Update() }
