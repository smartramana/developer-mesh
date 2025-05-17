module github.com/S-Corkum/devops-mcp/internal/observability

go 1.24

require (
	github.com/S-Corkum/devops-mcp/pkg/common v0.0.0
	go.opentelemetry.io/otel v1.14.0
	go.opentelemetry.io/otel/attribute v1.14.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.14.0
	go.opentelemetry.io/otel/propagation v1.14.0
	go.opentelemetry.io/otel/sdk v1.14.0
	go.opentelemetry.io/otel/sdk/resource v1.14.0
	go.opentelemetry.io/otel/trace v1.14.0
	google.golang.org/grpc v1.54.0
	google.golang.org/grpc/credentials/insecure v1.54.0
)

// Replace directives for local development
replace github.com/S-Corkum/devops-mcp/pkg/common => ../../pkg/common
