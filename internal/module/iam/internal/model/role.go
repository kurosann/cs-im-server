package model

import (
	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*Role)(nil)

type Role struct {
	base.BaseModel
	TenantID    string `gorm:"column:tenant_id;size:64;not null;uniqueIndex:idx_iam_roles_tenant_role"`
	RoleKey     string `gorm:"column:role_key;size:64;not null;uniqueIndex:idx_iam_roles_tenant_role"`
	Name        string `gorm:"column:name;size:255;not null"`
	Description string `gorm:"column:description;size:512"`
	Kind        string `gorm:"column:kind;size:16;not null;default:custom"`
	Status      string `gorm:"column:status;size:16;not null;default:active;index"`
	ScopesJSON  string `gorm:"column:scopes_json;size:4096;not null"`
}

func (*Role) TableName() string { return "iam_roles" }

func (r *Role) GetID() field.IID   { return r.BaseModel.GetID() }
func (r *Role) SetID(id field.IID) { r.BaseModel.SetID(id) }
func (r *Role) IDColumn() string   { return r.BaseModel.IDColumn() }
func (r *Role) Create()            { r.BaseModel.Create() }
func (r *Role) Update()            { r.BaseModel.Update() }
