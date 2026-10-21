package model

import (
	"encoding/json"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*Provider)(nil)

type Provider struct {
	base.BaseModel
	ProviderKey string `gorm:"column:provider_key;size:64;uniqueIndex" json:"providerId"`
	Name        string `gorm:"column:name;size:128;not null" json:"name"`
	Kind        string `gorm:"column:kind;size:64;not null" json:"kind"`
	Endpoint    string `gorm:"column:endpoint;size:512" json:"endpoint"`
	APIKeyEnv   string `gorm:"column:api_key_env;size:255" json:"apiKeyEnv"`
	Model       string `gorm:"column:model;size:128" json:"model"`
	Priority    int    `gorm:"column:priority;not null;default:50" json:"priority"`
	Status      string `gorm:"column:status;size:32;not null;default:active" json:"status"`
}

func (*Provider) TableName() string { return "translation_providers" }

func (p *Provider) GetID() field.IID   { return p.BaseModel.GetID() }
func (p *Provider) SetID(id field.IID) { p.BaseModel.SetID(id) }
func (p *Provider) IDColumn() string   { return p.BaseModel.IDColumn() }
func (p *Provider) Create()            { p.BaseModel.Create() }
func (p *Provider) Update()            { p.BaseModel.Update() }
func (p *Provider) MarshalJSON() ([]byte, error) {
	return json.Marshal(*p)
}
