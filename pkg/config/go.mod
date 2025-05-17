module github.com/S-Corkum/devops-mcp/pkg/config

go 1.24

require github.com/S-Corkum/devops-mcp/pkg/common v0.0.0

// Redirect to the common package
replace github.com/S-Corkum/devops-mcp/pkg/common => ../common
