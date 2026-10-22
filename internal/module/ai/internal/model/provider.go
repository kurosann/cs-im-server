package model

import (
	"encoding/json"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*AIProvider)(nil)

type AIProvider struct {
	base.BaseModel
	ProviderKey string `gorm:"column:provider_key;size:64;uniqueIndex" json:"providerId"`
	Name        string `gorm:"column:name;size:128;not null" json:"name"`
	Kind        string `gorm:"column:kind;size:64;not null" json:"kind"`
	Endpoint    string `gorm:"column:endpoint;size:512;not null" json:"endpoint"`
	APIKeyEnv   string `gorm:"column:api_key_env;size:255" json:"apiKeyEnv"`
	Model       string `gorm:"column:model;size:128;not null" json:"model"`
	Priority    int    `gorm:"column:priority;not null;default:100" json:"priority"`
	Status      string `gorm:"column:status;size:32;not null;default:active" json:"status"`
}

func (*AIProvider) TableName() string { return "ai_providers" }

func (p *AIProvider) GetID() field.IID   { return p.BaseModel.GetID() }
func (p *AIProvider) SetID(id field.IID) { p.BaseModel.SetID(id) }
func (p *AIProvider) IDColumn() string   { return p.BaseModel.IDColumn() }
func (p *AIProvider) Create()            { p.BaseModel.Create() }
func (p *AIProvider) Update()            { p.BaseModel.Update() }
func (p *AIProvider) MarshalJSON() ([]byte, error) {
	return json.Marshal(*p)
}
