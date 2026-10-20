package dto

import "time"

type LoginRequest struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

type SessionContext struct {
	Device string
	IP     string
}

type LoginResult struct {
	User  User  `json:"user"`
	Token Token `json:"token"`
}

type User struct {
	UserKey      string     `json:"userKey"`
	Account      string     `json:"account"`
	Email        string     `json:"email"`
	DisplayName  string     `json:"displayName"`
	TenantID     string     `json:"tenantId"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	LastLoginAt  *time.Time `json:"lastLoginAt,omitempty"`
	Online       bool       `json:"online"`
	Device       string     `json:"device,omitempty"`
	IP           string     `json:"ip,omitempty"`
	LastActiveAt *time.Time `json:"lastActiveAt,omitempty"`
}

type Token struct {
	Token     string     `json:"token"`
	TokenKey  string     `json:"tokenKey"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

type RegisterRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	TenantName  string `json:"tenantName"`
}

type CreateStaffRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

type CreateStaffResult struct {
	User            User   `json:"user"`
	InitialPassword string `json:"initialPassword"`
}

type UpdateStaffRequest struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}
