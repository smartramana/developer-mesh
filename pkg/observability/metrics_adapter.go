// Package observability provides unified observability functionality for the MCP system.
package observability

// The metrics_adapter.go file contained legacy compatibility adapters and converters that
// have been removed as part of the Go workspace migration. This included:
//
// - LegacyMetricsAdapter: An adapter between different metrics client implementations
// - LegacyClient: An interface for backward compatibility
// - LegacyConfig: A legacy metrics configuration format
// - Various conversion functions between legacy and current formats
//
// As backward compatibility is not a requirement for the Go workspace migration,
// these compatibility layers have been removed. All code should now use the
// standard MetricsClient interface directly.
//
// For metrics functionality, use these standard components instead:
// - MetricsClient interface defined in metrics.go
// - NewMetricsClient and NewMetricsClientWithOptions constructors
// - MetricsConfig defined in metrics.go
