package dto

type RegisterAppRequest struct {
	ServiceKey      string   `json:"serviceKey"`
	DisplayName     string   `json:"displayName"`
	Kind            string   `json:"kind"`
	Version         string   `json:"version"`
	Capabilities    []string `json:"capabilities"`
	LeaseTTLSeconds int      `json:"leaseTtlSeconds"`
}

type HeartbeatRequest struct {
	AppID        string   `json:"appId"`
	InstanceID   string   `json:"instanceId"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
	TTLSeconds   int      `json:"ttlSeconds"`
}
