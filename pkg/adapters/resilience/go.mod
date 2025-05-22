module github.com/S-Corkum/devops-mcp/pkg/adapters/resilience

go 1.24.2

require (
	github.com/S-Corkum/devops-mcp/pkg/adapters v0.0.0
	github.com/cenkalti/backoff/v4 v4.2.1
	github.com/sony/gobreaker v0.5.0
	golang.org/x/time v0.5.0
)

replace github.com/S-Corkum/devops-mcp/pkg/adapters => ../
