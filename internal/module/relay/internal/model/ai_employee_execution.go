package model

import (
	"encoding/json"
	"time"

	"vane-common/pkg/dataaccess/base"
	"vane-common/pkg/dataaccess/field"
	damodel "vane-common/pkg/dataaccess/model"

	"gorm.io/gorm"
)

var _ damodel.Table = (*AIEmployeeExecution)(nil)

type HistoryDiagnostics struct {
	Status         string  `json:"status"`
	RequestedCount int     `json:"requestedCount"`
	ReadCount      int     `json:"readCount"`
	Pages          int     `json:"pages"`
	ScannedCount   int     `json:"scannedCount"`
	Error          *string `json:"error"`
}

type AIEmployeeExecution struct {
	base.BaseModel
	ExecutionKey           string              `gorm:"column:execution_key;size:64;uniqueIndex" json:"-"`
	AccountKey             string              `gorm:"column:account_key;size:64;not null;index:idx_ai_employee_exec_owner,priority:1" json:"-"`
	EmployeeKey            string              `gorm:"column:employee_key;size:64;not null;index:idx_ai_employee_exec_owner,priority:2" json:"-"`
	EmployeeName           string              `gorm:"column:employee_name;size:50;not null" json:"-"`
	Platform               string              `gorm:"column:platform;size:64;not null" json:"-"`
	SessionAccountKey      string              `gorm:"column:session_account_key;size:255;not null" json:"-"`
	AccountDisplayName     *string             `gorm:"column:account_display_name;size:255" json:"-"`
	ProfileID              string              `gorm:"column:profile_id;size:255;not null" json:"-"`
	PeerID                 string              `gorm:"column:peer_id;size:255;not null" json:"-"`
	ConversationID         string              `gorm:"column:conversation_id;size:255;not null" json:"-"`
	PeerDisplayName        *string             `gorm:"column:peer_display_name;size:255" json:"-"`
	MessageIDsJSON         string              `gorm:"column:message_ids;type:text" json:"-"`
	MessageIDs             []string            `gorm:"-" json:"-"`
	MergedText             string              `gorm:"column:merged_text;type:text" json:"-"`
	HistoryCount           int                 `gorm:"column:history_count;not null;default:0" json:"-"`
	HistoryAvailable       bool                `gorm:"column:history_available;not null;default:false" json:"-"`
	HistoryTruncated       bool                `gorm:"column:history_truncated;not null;default:false" json:"-"`
	HistoryDiagnosticsJSON string              `gorm:"column:history_diagnostics;type:text" json:"-"`
	HistoryDiagnostics     *HistoryDiagnostics `gorm:"-" json:"-"`
	Status                 string              `gorm:"column:status;size:32;not null;index" json:"-"`
	Action                 *string             `gorm:"column:action;size:64" json:"-"`
	ReasonCode             *string             `gorm:"column:reason_code;size:64" json:"-"`
	Reason                 *string             `gorm:"column:reason;type:text" json:"-"`
	ReplyText              *string             `gorm:"column:reply_text;type:text" json:"-"`
	PromptTokens           int                 `gorm:"column:prompt_tokens;not null;default:0" json:"-"`
	CompletionTokens       int                 `gorm:"column:completion_tokens;not null;default:0" json:"-"`
	TotalTokens            int                 `gorm:"column:total_tokens;not null;default:0" json:"-"`
	Result                 *string             `gorm:"column:result;type:text" json:"-"`
	ErrorMessage           *string             `gorm:"column:error;type:text" json:"-"`
	CompletedAt            *time.Time          `gorm:"column:completed_at" json:"-"`
}

func (*AIEmployeeExecution) TableName() string { return "ai_employee_executions" }

func (e *AIEmployeeExecution) GetID() field.IID   { return e.BaseModel.GetID() }
func (e *AIEmployeeExecution) SetID(id field.IID) { e.BaseModel.SetID(id) }
func (e *AIEmployeeExecution) IDColumn() string   { return e.BaseModel.IDColumn() }
func (e *AIEmployeeExecution) Create()            { e.BaseModel.Create() }
func (e *AIEmployeeExecution) Update()            { e.BaseModel.Update() }

func (e *AIEmployeeExecution) BeforeSave(*gorm.DB) error {
	e.MessageIDsJSON = encodeStringList(e.MessageIDs)
	if e.HistoryDiagnostics == nil {
		e.HistoryDiagnosticsJSON = ""
		return nil
	}
	data, err := json.Marshal(e.HistoryDiagnostics)
	if err != nil {
		return err
	}
	e.HistoryDiagnosticsJSON = string(data)
	return nil
}

func (e *AIEmployeeExecution) AfterFind(*gorm.DB) error {
	e.MessageIDs = decodeStringList(e.MessageIDsJSON)
	if e.HistoryDiagnosticsJSON == "" {
		e.HistoryDiagnostics = nil
		return nil
	}
	var diagnostics HistoryDiagnostics
	if err := json.Unmarshal([]byte(e.HistoryDiagnosticsJSON), &diagnostics); err != nil {
		e.HistoryDiagnostics = nil
		return nil
	}
	e.HistoryDiagnostics = &diagnostics
	return nil
}
