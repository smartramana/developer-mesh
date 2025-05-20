package api

// PaginationOptions defines options for paginated API queries
type PaginationOptions struct {
	// Page is the current page number
	Page int
	// PerPage is the number of items per page
	PerPage int
	// MaxPages is the maximum number of pages to fetch
	MaxPages int
	// ResultHandler is called for each page of results
	ResultHandler func(page int, data interface{}) error
}

// DefaultPaginationOptions returns default pagination options
func DefaultPaginationOptions() *PaginationOptions {
	return &PaginationOptions{
		Page:     1,
		PerPage:  100,
		MaxPages: 10,
	}
}
