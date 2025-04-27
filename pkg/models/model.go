package models

type Model struct {
	ID       string `json:"id" db:"id"`
	TenantID string `json:"tenant_id" db:"tenant_id"`
	Name     string `json:"name" db:"name"`
}
