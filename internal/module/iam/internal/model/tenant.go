package model

import (
	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*Tenant)(nil)

type Tenant struct {
	base.BaseModel
	TenantKey string `gorm:"column:tenant_key;size:64;not null;uniqueIndex"`
	Name      string `gorm:"column:name;size:255;not null"`
	OwnerKey  string `gorm:"column:owner_key;size:64;not null;uniqueIndex"`
	Status    string `gorm:"column:status;size:32;not null;default:active;index"`
}

func (*Tenant) TableName() string { return "iam_tenants" }

func (t *Tenant) GetID() field.IID   { return t.BaseModel.GetID() }
func (t *Tenant) SetID(id field.IID) { t.BaseModel.SetID(id) }
func (t *Tenant) IDColumn() string   { return t.BaseModel.IDColumn() }
func (t *Tenant) Create()            { t.BaseModel.Create() }
func (t *Tenant) Update()            { t.BaseModel.Update() }
