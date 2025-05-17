module github.com/S-Corkum/devops-mcp/pkg/relationship

go 1.24

require github.com/S-Corkum/devops-mcp/pkg/models v0.0.0

// Redirect to the models package
replace github.com/S-Corkum/devops-mcp/pkg/models => ../models
