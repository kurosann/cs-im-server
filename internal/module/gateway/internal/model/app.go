package model

import (
	"encoding/json"
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"

	"gorm.io/gorm"
)

var _ damodel.Table = (*App)(nil)

type App struct {
	base.BaseModel
	AppKey           string     `gorm:"column:app_key;size:64;uniqueIndex" json:"appId"`
	TenantID         string     `gorm:"column:tenant_id;size:64;not null;uniqueIndex:idx_apps_tenant_service_key;index" json:"tenantId"`
	ServiceKey       string     `gorm:"column:service_key;size:255;not null;uniqueIndex:idx_apps_tenant_service_key" json:"serviceKey"`
	DisplayName      string     `gorm:"column:display_name;size:255;not null" json:"displayName"`
	Kind             string     `gorm:"column:kind;size:64" json:"kind"`
	Version          string     `gorm:"column:version;size:64" json:"version,omitempty"`
	CapabilitiesJSON string     `gorm:"column:capabilities;type:json" json:"-"`
	Capabilities     []string   `gorm:"-" json:"capabilities,omitempty"`
	Status           string     `gorm:"column:status;size:32;not null;default:active" json:"status"`
	LeaseTTLSeconds  int        `gorm:"column:lease_ttl_seconds;not null;default:60" json:"leaseTtlSeconds"`
	LastHeartbeatAt  *time.Time `gorm:"column:last_heartbeat_at" json:"lastHeartbeatAt"`
	LeaseExpiresAt   *time.Time `gorm:"column:lease_expires_at" json:"leaseExpiresAt"`
}

func (*App) TableName() string { return "apps" }

func (a *App) GetID() field.IID   { return a.BaseModel.GetID() }
func (a *App) SetID(id field.IID) { a.BaseModel.SetID(id) }
func (a *App) IDColumn() string   { return a.BaseModel.IDColumn() }
func (a *App) Create()            { a.BaseModel.Create() }
func (a *App) Update()            { a.BaseModel.Update() }

func (a *App) BeforeSave(*gorm.DB) error {
	data, _ := json.Marshal(a.Capabilities)
	a.CapabilitiesJSON = string(data)
	return nil
}

func (a *App) AfterFind(*gorm.DB) error {
	if a.CapabilitiesJSON != "" {
		_ = json.Unmarshal([]byte(a.CapabilitiesJSON), &a.Capabilities)
	}
	return nil
}

func (a *App) MarshalJSON() ([]byte, error) {
	type alias App
	return json.Marshal(alias{
		AppKey:          a.AppKey,
		TenantID:        a.TenantID,
		ServiceKey:      a.ServiceKey,
		DisplayName:     a.DisplayName,
		Kind:            a.Kind,
		Version:         a.Version,
		Capabilities:    a.Capabilities,
		Status:          a.Status,
		LeaseTTLSeconds: a.LeaseTTLSeconds,
		LastHeartbeatAt: a.LastHeartbeatAt,
		LeaseExpiresAt:  a.LeaseExpiresAt,
	})
}

func (a *App) UnmarshalJSON(data []byte) error {
	type alias App
	var v alias
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*a = App(v)
	return nil
}
