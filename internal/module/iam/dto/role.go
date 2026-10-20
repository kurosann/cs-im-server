package dto

type RoleInfo struct {
	RoleKey     string   `json:"roleKey"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Kind        string   `json:"kind"`
	Status      string   `json:"status"`
	Scopes      []string `json:"scopes"`
	Members     int64    `json:"members"`
}

type CreateRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Scopes      []string `json:"scopes"`
}

type UpdateRoleRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Scopes      []string `json:"scopes"`
}
