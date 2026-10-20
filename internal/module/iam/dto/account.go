package dto

import "time"

type AccountTokenInfo struct {
	TokenKey   string     `json:"tokenKey"`
	AccountKey string     `json:"accountKey"`
	Name       string     `json:"name"`
	TenantID   string     `json:"tenantId"`
	Kind       string     `json:"kind"`
	Scopes     []string   `json:"scopes"`
	Status     string     `json:"status"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

type IssueAccountTokenRequest struct {
	AccountKey string     `json:"accountKey"`
	Name       string     `json:"name"`
	TenantID   string     `json:"tenantId"`
	Kind       string     `json:"kind"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expiresAt"`
}

type IssueAccountTokenResult struct {
	Token  AccountTokenInfo `json:"token"`
	Secret string           `json:"secret"`
}

type ChannelCredentialInfo struct {
	AppID      string     `json:"appId"`
	Name       string     `json:"name"`
	TenantID   string     `json:"tenantId"`
	Status     string     `json:"status"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

type IssueChannelCredentialRequest struct {
	AppID     string     `json:"appId"`
	Name      string     `json:"name"`
	ExpiresAt *time.Time `json:"expiresAt"`
}

type IssueChannelCredentialResult struct {
	Credential ChannelCredentialInfo `json:"credential"`
	Secret     string                `json:"secret"`
}
