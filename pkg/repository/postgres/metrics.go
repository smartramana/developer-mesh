package postgres

import (
	"github.com/prometheus/client_golang/prometheus"
)

// initializeMetrics creates and registers repository metrics
func initializeMetrics() *repositoryMetrics {
	m := &repositoryMetrics{
		queries: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "repository_queries_total",
				Help: "Total number of repository queries",
			},
			[]string{"operation", "status"},
		),
		queryDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "repository_query_duration_seconds",
				Help:    "Query duration in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5},
			},
			[]string{"operation"},
		),
		cacheHits: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "repository_cache_hits_total",
				Help: "Total number of cache hits",
			},
			[]string{"level"},
		),
		cacheMisses: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "repository_cache_misses_total",
				Help: "Total number of cache misses",
			},
			[]string{"level"},
		),
		errors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "repository_errors_total",
				Help: "Total number of repository errors",
			},
			[]string{"operation", "error_type"},
		),
		poolStats: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "repository_pool_connections",
				Help: "Database connection pool statistics",
			},
			[]string{"pool", "state"},
		),
	}

	// Register metrics
	prometheus.MustRegister(
		m.queries,
		m.queryDuration,
		m.cacheHits,
		m.cacheMisses,
		m.errors,
		m.poolStats,
	)

	return m
}