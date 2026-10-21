package dto

type CreateProviderRequest struct {
	Name        string `json:"name"`
	ProviderKey string `json:"providerId"`
	Kind        string `json:"kind"`
	Endpoint    string `json:"endpoint"`
	APIKeyEnv   string `json:"apiKeyEnv"`
	Model       string `json:"model"`
	Priority    int    `json:"priority"`
	IsActive    bool   `json:"isActive"`
}

type UpdateProviderRequest struct {
	Name      string `json:"name"`
	Endpoint  string `json:"endpoint"`
	APIKeyEnv string `json:"apiKeyEnv"`
	Model     string `json:"model"`
	Priority  int    `json:"priority"`
}

type GenerateRequest struct {
	Prompt     string `json:"prompt"`
	System     string `json:"system"`
	ProviderID string `json:"providerId"`
}

type GenerateResult struct {
	Text         string `json:"text"`
	ProviderID   string `json:"providerId,omitempty"`
	ProviderKind string `json:"providerKind,omitempty"`
	ModelID      string `json:"modelId,omitempty"`
	Priority     int    `json:"priority"`
}
