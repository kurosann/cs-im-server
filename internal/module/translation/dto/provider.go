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

type ProviderTestRequest struct {
	Text       string `json:"text"`
	TargetLang string `json:"targetLang"`
}
