package model

import (
	"encoding/json"
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"
)

var _ damodel.Table = (*AIConfig)(nil)

type AIConfig struct {
	base.BaseModel
	Prompt              string `gorm:"column:prompt;type:text" json:"prompt"`
	HistoryMessageCount int    `gorm:"column:history_message_count;not null;default:20" json:"historyMessageCount"`
	AutoReplyEnabled    bool   `gorm:"column:auto_reply_enabled;not null;default:true" json:"autoReplyEnabled"`
	ReportingEnabled    bool   `gorm:"column:reporting_enabled;not null;default:true" json:"reportingEnabled"`
}

func (*AIConfig) TableName() string { return "ai_config" }

func (c *AIConfig) GetID() field.IID   { return c.BaseModel.GetID() }
func (c *AIConfig) SetID(id field.IID) { c.BaseModel.SetID(id) }
func (c *AIConfig) IDColumn() string   { return c.BaseModel.IDColumn() }
func (c *AIConfig) Create()            { c.BaseModel.Create() }
func (c *AIConfig) Update()            { c.BaseModel.Update() }

func (c *AIConfig) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Prompt              string `json:"prompt"`
		HistoryMessageCount int    `json:"historyMessageCount"`
		AutoReplyEnabled    bool   `json:"autoReplyEnabled"`
		ReportingEnabled    bool   `json:"reportingEnabled"`
	}{
		Prompt:              c.Prompt,
		HistoryMessageCount: c.HistoryMessageCount,
		AutoReplyEnabled:    c.AutoReplyEnabled,
		ReportingEnabled:    c.ReportingEnabled,
	})
}

func (c *AIConfig) UnmarshalJSON(data []byte) error {
	var v struct {
		Prompt              string `json:"prompt"`
		HistoryMessageCount int    `json:"historyMessageCount"`
		AutoReplyEnabled    bool   `json:"autoReplyEnabled"`
		ReportingEnabled    bool   `json:"reportingEnabled"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	c.Prompt = v.Prompt
	c.HistoryMessageCount = v.HistoryMessageCount
	c.AutoReplyEnabled = v.AutoReplyEnabled
	c.ReportingEnabled = v.ReportingEnabled
	return nil
}

var _ damodel.Table = (*AIExecution)(nil)

type AIExecution struct {
	base.BaseModel
	ExecutionKey    string `gorm:"column:execution_key;size:64;uniqueIndex" json:"executionId"`
	ConversationKey string `gorm:"column:conversation_key;size:64;not null;index" json:"conversationId"`
	MessageKey      string `gorm:"column:message_key;size:64;not null" json:"messageId"`
	Action          string `gorm:"column:action;size:64;not null" json:"action"`
	Result          string `gorm:"column:result;type:text" json:"result,omitempty"`
}

func (*AIExecution) TableName() string { return "ai_executions" }

func (e *AIExecution) GetID() field.IID   { return e.BaseModel.GetID() }
func (e *AIExecution) SetID(id field.IID) { e.BaseModel.SetID(id) }
func (e *AIExecution) IDColumn() string   { return e.BaseModel.IDColumn() }
func (e *AIExecution) Create()            { e.BaseModel.Create() }
func (e *AIExecution) Update()            { e.BaseModel.Update() }

func (e *AIExecution) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		ExecutionKey    string `json:"executionId"`
		ConversationKey string `json:"conversationId"`
		MessageKey      string `json:"messageId"`
		Action          string `json:"action"`
		Result          string `json:"result,omitempty"`
		CreatedAt       string `json:"createdAt"`
	}{
		ExecutionKey:    e.ExecutionKey,
		ConversationKey: e.ConversationKey,
		MessageKey:      e.MessageKey,
		Action:          e.Action,
		Result:          e.Result,
		CreatedAt:       e.BaseModel.CreateAt.Format(time.RFC3339Nano),
	})
}
