package model

import (
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*User)(nil)

type User struct {
	base.BaseModel
	UserKey      string     `gorm:"column:user_key;size:64;not null;uniqueIndex"`
	Account      string     `gorm:"column:account;size:64;not null;uniqueIndex"`
	Email        string     `gorm:"column:email;size:255;not null;uniqueIndex"`
	DisplayName  string     `gorm:"column:display_name;size:255"`
	PasswordHash string     `gorm:"column:password_hash;size:255;not null"`
	TenantID     string     `gorm:"column:tenant_id;size:64;not null;index"`
	Role         string     `gorm:"column:role;size:32;not null;default:agent;index"`
	Status       string     `gorm:"column:status;size:32;not null;default:active"`
	LastLoginAt  *time.Time `gorm:"column:last_login_at"`
}

func (*User) TableName() string { return "iam_users" }

func (u *User) GetID() field.IID   { return u.BaseModel.GetID() }
func (u *User) SetID(id field.IID) { u.BaseModel.SetID(id) }
func (u *User) IDColumn() string   { return u.BaseModel.IDColumn() }
func (u *User) Create()            { u.BaseModel.Create() }
func (u *User) Update()            { u.BaseModel.Update() }
