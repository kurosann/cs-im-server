package dto

type AuditEntry struct {
	Event  string `json:"event"`
	Action string `json:"action"`
	Object string `json:"object"`
	Result string `json:"result"`
}

type AuditContext struct {
	Device string
	IP     string
}

type AuditFilter struct {
	Keyword  string
	Action   string
	Result   string
	Page     int
	PageSize int
}

type AuditLog struct {
	ID        int64  `json:"id"`
	Event     string `json:"event"`
	Action    string `json:"action"`
	Operator  string `json:"operator"`
	Object    string `json:"object"`
	Result    string `json:"result"`
	IP        string `json:"ip"`
	Device    string `json:"device"`
	CreatedAt string `json:"createdAt"`
}

type AuditPage struct {
	Items []AuditLog `json:"items"`
	Total int64      `json:"total"`
}
