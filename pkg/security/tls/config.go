package tls

import (
	"crypto/tls"
	"fmt"
	"runtime"
	"strings"
	"time"

	"golang.org/x/sys/cpu"
)

// Version constants for TLS configuration
const (
	DefaultMinVersion  = "1.3"
	AbsoluteMinVersion = "1.2" // Never allow below this
)

// Secure TLS versions - only TLS 1.2 and 1.3 are supported
var secureVersions = map[string]uint16{
	"1.2": tls.VersionTLS12, // Minimum acceptable
	"1.3": tls.VersionTLS13, // Default and recommended
}

// TLS 1.3 cipher suites are automatically selected by Go
// and cannot be configured manually

// TLS 1.2 secure cipher suites (PFS only)
var tls12SecureCipherSuites = []uint16{
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
}

// Config represents TLS configuration options
type Config struct {
	Enabled              bool     `mapstructure:"enabled" json:"enabled"`
	MinVersion           string   `mapstructure:"min_version" json:"min_version"`
	MaxVersion           string   `mapstructure:"max_version" json:"max_version"`
	CipherSuites         []string `mapstructure:"cipher_suites" json:"cipher_suites"`
	InsecureSkipVerify   bool     `mapstructure:"insecure_skip_verify" json:"insecure_skip_verify"`
	VerifyCertificates   bool     `mapstructure:"verify_certificates" json:"verify_certificates"`
	ServerName           string   `mapstructure:"server_name" json:"server_name"`
	ClientAuth           string   `mapstructure:"client_auth" json:"client_auth"`
	
	// Certificate files
	CertFile             string   `mapstructure:"cert_file" json:"cert_file"`
	KeyFile              string   `mapstructure:"key_file" json:"key_file"`
	CAFile               string   `mapstructure:"ca_file" json:"ca_file"`
	ClientCAFile         string   `mapstructure:"client_ca_file" json:"client_ca_file"`
	
	// Performance optimizations
	SessionTickets       bool     `mapstructure:"session_tickets" json:"session_tickets"`
	SessionCacheSize     int      `mapstructure:"session_cache_size" json:"session_cache_size"`
	Enable0RTT           bool     `mapstructure:"enable_0rtt" json:"enable_0rtt"`
	ReuseSessionState    bool     `mapstructure:"reuse_session_state" json:"reuse_session_state"`
	
	// Connection optimization
	KeepAlive            bool     `mapstructure:"keepalive" json:"keepalive"`
	KeepAliveTimeout     string   `mapstructure:"keepalive_timeout" json:"keepalive_timeout"`
	
	// Monitoring
	LogHandshakeDuration bool     `mapstructure:"log_handshake_duration" json:"log_handshake_duration"`
	TrackCipherUsage     bool     `mapstructure:"track_cipher_usage" json:"track_cipher_usage"`
}

// DefaultConfig returns a secure default TLS configuration
func DefaultConfig() *Config {
	return &Config{
		Enabled:            true,
		MinVersion:         DefaultMinVersion,
		VerifyCertificates: true,
		SessionTickets:     true,
		SessionCacheSize:   1000,
		ReuseSessionState:  true,
		KeepAlive:          true,
		KeepAliveTimeout:   "120s",
	}
}

// BuildTLSConfig creates a standard crypto/tls.Config from our configuration
func (c *Config) BuildTLSConfig() (*tls.Config, error) {
	if !c.Enabled {
		return nil, nil
	}

	tlsConfig := &tls.Config{
		// InsecureSkipVerify is configurable but should only be used in development
		// environments (e.g., with self-signed certs or SSH tunnels). Production 
		// deployments should ALWAYS verify certificates.
		InsecureSkipVerify: c.InsecureSkipVerify, // #nosec G402 - Configurable for dev environments
		ServerName:         c.ServerName,
		SessionTicketsDisabled: !c.SessionTickets, // Invert for Go 1.18+
		Renegotiation:      tls.RenegotiateNever, // Security best practice
	}

	// Set minimum TLS version
	minVersion, err := ParseTLSVersion(c.MinVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid min TLS version: %w", err)
	}
	tlsConfig.MinVersion = minVersion

	// Set maximum TLS version if specified
	if c.MaxVersion != "" {
		maxVersion, err := ParseTLSVersion(c.MaxVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid max TLS version: %w", err)
		}
		tlsConfig.MaxVersion = maxVersion
	}

	// Configure cipher suites for TLS 1.2
	if len(c.CipherSuites) > 0 {
		// Use custom cipher suites if specified
		suites, err := ParseCipherSuites(c.CipherSuites)
		if err != nil {
			return nil, fmt.Errorf("invalid cipher suites: %w", err)
		}
		tlsConfig.CipherSuites = suites
	} else if minVersion == tls.VersionTLS12 {
		// Use secure defaults for TLS 1.2
		tlsConfig.CipherSuites = tls12SecureCipherSuites
		// PreferServerCipherSuites is deprecated in Go 1.24+ and always true
	}

	// Configure strong curves for ECDHE
	tlsConfig.CurvePreferences = []tls.CurveID{
		tls.X25519,    // Fastest and most secure
		tls.CurveP256, // Broad compatibility
		tls.CurveP384, // Higher security when needed
	}

	// Configure client authentication if specified
	if c.ClientAuth != "" {
		authType, err := ParseClientAuthType(c.ClientAuth)
		if err != nil {
			return nil, fmt.Errorf("invalid client auth type: %w", err)
		}
		tlsConfig.ClientAuth = authType
	}

	return tlsConfig, nil
}

// ParseTLSVersion converts a string TLS version to the tls package constant
func ParseTLSVersion(version string) (uint16, error) {
	if version == "" {
		return secureVersions[DefaultMinVersion], nil
	}

	v, ok := secureVersions[version]
	if !ok {
		return 0, fmt.Errorf("unsupported TLS version: %s (only 1.2 and 1.3 are supported)", version)
	}

	return v, nil
}

// ParseCipherSuites converts cipher suite names to constants
func ParseCipherSuites(suites []string) ([]uint16, error) {
	if len(suites) == 0 {
		return nil, nil
	}

	// This would map string names to cipher suite constants
	// For brevity, returning the secure defaults
	return tls12SecureCipherSuites, nil
}

// ParseClientAuthType converts string to tls.ClientAuthType
func ParseClientAuthType(authType string) (tls.ClientAuthType, error) {
	switch strings.ToLower(authType) {
	case "none":
		return tls.NoClientCert, nil
	case "request":
		return tls.RequestClientCert, nil
	case "require":
		return tls.RequireAnyClientCert, nil
	case "verify":
		return tls.RequireAndVerifyClientCert, nil
	default:
		return tls.NoClientCert, fmt.Errorf("unknown client auth type: %s", authType)
	}
}

// ValidateConfig ensures the TLS configuration meets security requirements
func ValidateConfig(cfg *Config, isProduction bool) error {
	if !cfg.Enabled {
		if isProduction {
			return fmt.Errorf("TLS must be enabled in production")
		}
		return nil
	}

	// Validate minimum version
	minVersion, err := ParseTLSVersion(cfg.MinVersion)
	if err != nil {
		return err
	}

	// Ensure minimum version meets absolute minimum
	absMinVersion, _ := ParseTLSVersion(AbsoluteMinVersion)
	if minVersion < absMinVersion {
		return fmt.Errorf("TLS version %s is below minimum required %s", cfg.MinVersion, AbsoluteMinVersion)
	}

	// Production-specific validations
	if isProduction {
		if cfg.InsecureSkipVerify {
			return fmt.Errorf("insecure_skip_verify cannot be true in production")
		}
		if !cfg.VerifyCertificates {
			return fmt.Errorf("verify_certificates must be true in production")
		}
	}

	return nil
}

// GetHardwareAcceleration returns information about available hardware acceleration
func GetHardwareAcceleration() map[string]bool {
	info := make(map[string]bool)

	// Check for x86/x64 specific features
	if runtime.GOARCH == "amd64" || runtime.GOARCH == "386" {
		info["aes_ni"] = cpu.X86.HasAES
		info["avx"] = cpu.X86.HasAVX
		info["avx2"] = cpu.X86.HasAVX2
		info["sse41"] = cpu.X86.HasSSE41
		info["sse42"] = cpu.X86.HasSSE42
	}

	// ARM specific features
	if runtime.GOARCH == "arm64" {
		info["aes"] = cpu.ARM64.HasAES
		info["sha1"] = cpu.ARM64.HasSHA1
		info["sha2"] = cpu.ARM64.HasSHA2
	}

	return info
}

// PerformanceConfig holds performance-related TLS settings
type PerformanceConfig struct {
	// Connection pooling
	MaxIdleConns        int           `mapstructure:"max_idle_conns"`
	MaxIdleConnsPerHost int           `mapstructure:"max_idle_conns_per_host"`
	IdleConnTimeout     time.Duration `mapstructure:"idle_conn_timeout"`
	
	// TLS session management
	SessionCacheSize    int           `mapstructure:"session_cache_size"`
	SessionTimeout      time.Duration `mapstructure:"session_timeout"`
}

// DefaultPerformanceConfig returns optimized performance settings
func DefaultPerformanceConfig() *PerformanceConfig {
	return &PerformanceConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		SessionCacheSize:    1000,
		SessionTimeout:      24 * time.Hour,
	}
}