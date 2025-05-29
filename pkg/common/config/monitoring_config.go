package config

// MonitoringConfig holds monitoring configuration
type MonitoringConfig struct {
	Prometheus PrometheusConfig `yaml:"prometheus"`
	Logging    LoggingConfig    `yaml:"logging"`
}

// PrometheusConfig holds Prometheus configuration
type PrometheusConfig struct {
	Enabled       bool                `yaml:"enabled"`
	Path          string              `yaml:"path"`
	VectorMetrics VectorMetricsConfig `yaml:"vector_metrics"`
}

// VectorMetricsConfig holds configuration for vector-specific metrics
type VectorMetricsConfig struct {
	Enabled           bool      `yaml:"enabled"`
	CollectHistograms bool      `yaml:"collect_histograms"`
	Percentiles       []float64 `yaml:"percentiles"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level    string `yaml:"level"`
	Format   string `yaml:"format"`
	Output   string `yaml:"output"`
	FilePath string `yaml:"file_path"`
}

// GetDefaultMonitoringConfig returns default monitoring configuration
func GetDefaultMonitoringConfig() MonitoringConfig {
	return MonitoringConfig{
		Prometheus: PrometheusConfig{
			Enabled: true,
			Path:    "/metrics",
			VectorMetrics: VectorMetricsConfig{
				Enabled:           true,
				CollectHistograms: true,
				Percentiles:       []float64{0.5, 0.9, 0.95, 0.99},
			},
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		},
	}
}
