module github.com/S-Corkum/devops-mcp/apps/mockserver

go 1.24

// External dependencies
require github.com/stretchr/testify v1.8.4

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// Replace directives for local development
replace (
	github.com/S-Corkum/devops-mcp/pkg/common => ../../pkg/common
	github.com/S-Corkum/devops-mcp/pkg/models => ../../pkg/models
)

// Mock server for testing external service integrations
