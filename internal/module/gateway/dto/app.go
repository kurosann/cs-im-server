package dto

import "time"

type RegisterAppRequest struct {
	ServiceKey      string   `json:"serviceKey"`
	DisplayName     string   `json:"displayName"`
	Kind            string   `json:"kind"`
	Version         string   `json:"version"`
	Capabilities    []string `json:"capabilities"`
	LeaseTTLSeconds int      `json:"leaseTtlSeconds"`
}

type App struct {
	AppKey          string     `json:"appId"`
	TenantID        string     `json:"tenantId"`
	ServiceKey      string     `json:"serviceKey"`
	DisplayName     string     `json:"displayName"`
	Kind            string     `json:"kind"`
	Version         string     `json:"version,omitempty"`
	Capabilities    []string   `json:"capabilities,omitempty"`
	Status          string     `json:"status"`
	LeaseTTLSeconds int        `json:"leaseTtlSeconds"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt"`
	LeaseExpiresAt  *time.Time `json:"leaseExpiresAt"`
}

type HeartbeatRequest struct {
	AppID        string   `json:"appId"`
	InstanceID   string   `json:"instanceId"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
	TTLSeconds   int      `json:"ttlSeconds"`
}
