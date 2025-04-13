package models

// Database query options
type QueryOptions struct {
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	SortBy string `json:"sortBy"`
	Order  string `json:"order"`
}

// GitHub query types are defined in github.go
