package api

// RestPaginationOptions defines options for paginated REST API queries
type RestPaginationOptions struct {
	// Page is the current page number
	Page int
	// PerPage is the number of items per page
	PerPage int
	// MaxPages is the maximum number of pages to fetch
	MaxPages int
	// ResultHandler is called for each page of results
	ResultHandler func(page int, data map[string]interface{}) error
}

// DefaultRestPaginationOptions returns default pagination options for REST APIs
func DefaultRestPaginationOptions() *RestPaginationOptions {
	return &RestPaginationOptions{
		Page:     1,
		PerPage:  100,
		MaxPages: 10,
	}
}
