package model

import (
	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*AuditLog)(nil)

type AuditLog struct {
	base.BaseModel
	TenantID  string `gorm:"column:tenant_id;size:64;not null;index"`
	ActorKey  string `gorm:"column:actor_key;size:64;not null;index"`
	ActorName string `gorm:"column:actor_name;size:255"`
	ActorKind string `gorm:"column:actor_kind;size:32;not null;default:human"`
	Event     string `gorm:"column:event;size:64;not null;index"`
	Action    string `gorm:"column:action;size:64;not null;index"`
	Object    string `gorm:"column:object;size:255;not null"`
	Result    string `gorm:"column:result;size:16;not null;default:success;index"`
	IP        string `gorm:"column:ip;size:64"`
	Device    string `gorm:"column:device;size:255"`
}

func (*AuditLog) TableName() string { return "iam_audit_logs" }

func (a *AuditLog) GetID() field.IID   { return a.BaseModel.GetID() }
func (a *AuditLog) SetID(id field.IID) { a.BaseModel.SetID(id) }
func (a *AuditLog) IDColumn() string   { return a.BaseModel.IDColumn() }
func (a *AuditLog) Create()            { a.BaseModel.Create() }
func (a *AuditLog) Update()            { a.BaseModel.Update() }
