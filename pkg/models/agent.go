package models

type Agent struct {
	ID       string `json:"id" db:"id"`
	TenantID string `json:"tenant_id" db:"tenant_id"`
	Name     string `json:"name" db:"name"`
	ModelID  string `json:"model_id" db:"model_id"`
}
