package model

import (
	"encoding/json"
	"strings"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"

	"gorm.io/gorm"
)

var _ damodel.Table = (*KnowledgeGroup)(nil)

type KnowledgeGroup struct {
	base.BaseModel
	GroupKey string `gorm:"column:group_key;size:64;not null;uniqueIndex:uidx_knowledge_group,priority:2"`
	TenantID string `gorm:"column:tenant_id;size:64;not null;index;uniqueIndex:uidx_knowledge_group,priority:1"`
	Name     string `gorm:"column:name;size:100;not null"`
}

func (*KnowledgeGroup) TableName() string { return "knowledge_groups" }

func (g *KnowledgeGroup) GetID() field.IID   { return g.BaseModel.GetID() }
func (g *KnowledgeGroup) SetID(id field.IID) { g.BaseModel.SetID(id) }
func (g *KnowledgeGroup) IDColumn() string   { return g.BaseModel.IDColumn() }
func (g *KnowledgeGroup) Create()            { g.BaseModel.Create() }
func (g *KnowledgeGroup) Update()            { g.BaseModel.Update() }

var _ damodel.Table = (*KnowledgeBase)(nil)

type KnowledgeBase struct {
	base.BaseModel
	BaseKey     string `gorm:"column:base_key;size:64;not null;uniqueIndex:uidx_knowledge_base,priority:2"`
	TenantID    string `gorm:"column:tenant_id;size:64;not null;index;uniqueIndex:uidx_knowledge_base,priority:1"`
	GroupKey    string `gorm:"column:group_key;size:64;not null;index"`
	Name        string `gorm:"column:name;size:100;not null"`
	Product     string `gorm:"column:product;size:100"`
	Description string `gorm:"column:description;size:255"`
}

func (*KnowledgeBase) TableName() string { return "knowledge_bases" }

func (b *KnowledgeBase) GetID() field.IID   { return b.BaseModel.GetID() }
func (b *KnowledgeBase) SetID(id field.IID) { b.BaseModel.SetID(id) }
func (b *KnowledgeBase) IDColumn() string   { return b.BaseModel.IDColumn() }
func (b *KnowledgeBase) Create()            { b.BaseModel.Create() }
func (b *KnowledgeBase) Update()            { b.BaseModel.Update() }

var _ damodel.Table = (*KnowledgeDoc)(nil)

type KnowledgeDoc struct {
	base.BaseModel
	DocKey       string   `gorm:"column:doc_key;size:64;not null;uniqueIndex:uidx_knowledge_doc,priority:2"`
	TenantID     string   `gorm:"column:tenant_id;size:64;not null;index;uniqueIndex:uidx_knowledge_doc,priority:1"`
	BaseKey      string   `gorm:"column:base_key;size:64;not null;index"`
	Title        string   `gorm:"column:title;size:200;not null"`
	DocType      string   `gorm:"column:doc_type;size:32;not null"`
	Language     string   `gorm:"column:language;size:32;not null"`
	Status       string   `gorm:"column:status;size:32;not null;index"`
	Answer       string   `gorm:"column:answer;type:text"`
	Hits         int      `gorm:"column:hits;not null;default:0"`
	ChannelsJSON string   `gorm:"column:channels;type:text"`
	Channels     []string `gorm:"-"`
	EffectiveAt  string   `gorm:"column:effective_at;size:32"`
	UpdatedOn    string   `gorm:"column:updated_on;size:32"`
	Source       string   `gorm:"column:source;size:64"`
}

func (*KnowledgeDoc) TableName() string { return "knowledge_docs" }

func (d *KnowledgeDoc) GetID() field.IID   { return d.BaseModel.GetID() }
func (d *KnowledgeDoc) SetID(id field.IID) { d.BaseModel.SetID(id) }
func (d *KnowledgeDoc) IDColumn() string   { return d.BaseModel.IDColumn() }
func (d *KnowledgeDoc) Create()            { d.BaseModel.Create() }
func (d *KnowledgeDoc) Update()            { d.BaseModel.Update() }

func (d *KnowledgeDoc) BeforeSave(*gorm.DB) error {
	if d.Channels == nil {
		d.Channels = []string{}
	}
	data, err := json.Marshal(d.Channels)
	if err != nil {
		return err
	}
	d.ChannelsJSON = string(data)
	return nil
}

func (d *KnowledgeDoc) AfterFind(*gorm.DB) error {
	d.Channels = decodeKnowledgeChannels(d.ChannelsJSON)
	return nil
}

func decodeKnowledgeChannels(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil {
		return []string{}
	}
	return values
}
