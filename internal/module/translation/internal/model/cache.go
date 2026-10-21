package model

import (
	"encoding/json"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*Cache)(nil)

type Cache struct {
	base.BaseModel
	CacheKey     string `gorm:"column:cache_key;size:64;uniqueIndex" json:"cacheKey"`
	TextHash     string `gorm:"column:text_hash;size:64;not null;index" json:"textHash"`
	SourceLang   string `gorm:"column:source_lang;size:32" json:"sourceLang"`
	TargetLang   string `gorm:"column:target_lang;size:32;not null;index" json:"targetLang"`
	Result       string `gorm:"column:result;type:text;not null" json:"result"`
	ProviderKind string `gorm:"column:provider_kind;size:32;not null" json:"providerKind"`
	ProviderID   string `gorm:"column:provider_id;size:64;not null" json:"providerId"`
	ModelID      string `gorm:"column:model_id;size:128" json:"modelId"`
	Priority     int    `gorm:"column:priority;not null" json:"priority"`
}

func (*Cache) TableName() string { return "translation_cache" }

func (c *Cache) GetID() field.IID   { return c.BaseModel.GetID() }
func (c *Cache) SetID(id field.IID) { c.BaseModel.SetID(id) }
func (c *Cache) IDColumn() string   { return c.BaseModel.IDColumn() }
func (c *Cache) Create()            { c.BaseModel.Create() }
func (c *Cache) Update()            { c.BaseModel.Update() }
func (c *Cache) MarshalJSON() ([]byte, error) {
	return json.Marshal(*c)
}
