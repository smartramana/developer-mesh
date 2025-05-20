// Package metrics provides metrics collection functionality for the MCP system.
//
// Deprecated: This package is deprecated and will be removed in a future release.
// Use github.com/S-Corkum/devops-mcp/pkg/observability instead, which provides
// a unified interface for metrics, logging, and tracing.
//
// Migration Guide:
// 1. Import the new package:
//    import "github.com/S-Corkum/devops-mcp/pkg/observability"
//
// 2. Replace client creation:
//    Old: metrics.NewClient(cfg)
//    New: observability.NewClientFromInternal(cfg)
//       or observability.DefaultMetricsClient
//
// 3. For direct MetricsClient usage:
//    observability.NewCommonMetricsClient(observability.DefaultMetricsClient)
//
// The observability package provides additional functionality beyond metrics,
// including logging and tracing with a consistent interface.
package metrics

// Nothing else is needed in this file as it only serves to provide package-level deprecation notice
