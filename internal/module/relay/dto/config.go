package dto

type CreateChannelRequest struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Endpoint string         `json:"endpoint"`
	Config   map[string]any `json:"config"`
}

type UpdateChannelRequest struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Endpoint string         `json:"endpoint"`
	Config   map[string]any `json:"config"`
}

type CreateWebhookRequest struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Events   []string `json:"events"`
	IsActive bool     `json:"isActive"`
}

type UpdateWebhookRequest struct {
	Name     string   `json:"name"`
	URL      string   `json:"url"`
	Events   []string `json:"events"`
	IsActive bool     `json:"isActive"`
}
