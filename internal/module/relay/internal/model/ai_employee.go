package model

import (
	"encoding/json"
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"

	"gorm.io/gorm"
)

var _ damodel.Table = (*AIEmployee)(nil)

type AIEmployee struct {
	base.BaseModel
	EmployeeKey         string     `gorm:"column:employee_key;size:64;uniqueIndex" json:"employeeId"`
	AccountKey          string     `gorm:"column:account_key;size:64;not null;index;uniqueIndex:uidx_ai_employee_account_name" json:"-"`
	TenantID            string     `gorm:"column:tenant_id;size:64;index" json:"-"`
	Name                string     `gorm:"column:name;size:50;not null" json:"name"`
	NameKey             string     `gorm:"column:name_key;size:100;not null;uniqueIndex:uidx_ai_employee_account_name" json:"-"`
	Persona             string     `gorm:"column:persona;size:255" json:"persona"`
	Mode                string     `gorm:"column:mode;size:32;not null;default:basic" json:"mode"`
	Prompt              string     `gorm:"column:prompt;type:text" json:"prompt"`
	HistoryMessageCount int        `gorm:"column:history_message_count;not null;default:10" json:"history"`
	AutoReplyEnabled    bool       `gorm:"column:auto_reply_enabled;not null;default:true" json:"autoReply"`
	IsDefault           bool       `gorm:"column:is_default;not null;default:false" json:"isDefault"`
	Status              string     `gorm:"column:status;size:32;not null;default:paused" json:"status"`
	SelectedDocIDsJSON  string     `gorm:"column:selected_doc_ids;type:text" json:"-"`
	SelectedDocIDs      []string   `gorm:"-" json:"selectedDocIds"`
	ToolsJSON           string     `gorm:"column:tools;type:text" json:"-"`
	Tools               []string   `gorm:"-" json:"tools"`
	DeletedAt           *time.Time `gorm:"column:deleted_at;index" json:"-"`
}

func (*AIEmployee) TableName() string { return "ai_employees" }

func (e *AIEmployee) GetID() field.IID   { return e.BaseModel.GetID() }
func (e *AIEmployee) SetID(id field.IID) { e.BaseModel.SetID(id) }
func (e *AIEmployee) IDColumn() string   { return e.BaseModel.IDColumn() }
func (e *AIEmployee) Create()            { e.BaseModel.Create() }
func (e *AIEmployee) Update()            { e.BaseModel.Update() }

func (e *AIEmployee) BeforeSave(*gorm.DB) error {
	e.SelectedDocIDsJSON = encodeStringList(e.SelectedDocIDs)
	e.ToolsJSON = encodeStringList(e.Tools)
	return nil
}

func (e *AIEmployee) AfterFind(*gorm.DB) error {
	e.SelectedDocIDs = decodeStringList(e.SelectedDocIDsJSON)
	e.Tools = decodeStringList(e.ToolsJSON)
	return nil
}

var _ damodel.Table = (*AIEmployeeBinding)(nil)

type AIEmployeeBinding struct {
	base.BaseModel
	AccountKey      string `gorm:"column:account_key;size:64;not null;uniqueIndex:uidx_ai_employee_binding" json:"-"`
	TenantID        string `gorm:"column:tenant_id;size:64;index" json:"-"`
	EmployeeKey     string `gorm:"column:employee_key;size:64;not null;index" json:"employeeId"`
	Platform        string `gorm:"column:platform;size:64;not null;uniqueIndex:uidx_ai_employee_binding" json:"platform"`
	BoundAccountKey string `gorm:"column:account_key_binding;size:255;not null;uniqueIndex:uidx_ai_employee_binding" json:"accountKey"`
}

func (*AIEmployeeBinding) TableName() string { return "ai_employee_bindings" }

func (b *AIEmployeeBinding) GetID() field.IID   { return b.BaseModel.GetID() }
func (b *AIEmployeeBinding) SetID(id field.IID) { b.BaseModel.SetID(id) }
func (b *AIEmployeeBinding) IDColumn() string   { return b.BaseModel.IDColumn() }
func (b *AIEmployeeBinding) Create()            { b.BaseModel.Create() }
func (b *AIEmployeeBinding) Update()            { b.BaseModel.Update() }

func encodeStringList(values []string) string {
	if values == nil {
		values = []string{}
	}
	data, _ := json.Marshal(values)
	return string(data)
}

func decodeStringList(raw string) []string {
	if raw == "" {
		return []string{}
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil || values == nil {
		return []string{}
	}
	return values
}
