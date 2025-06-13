package tls

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTLSVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    uint16
		wantErr bool
	}{
		{
			name:    "TLS 1.2",
			version: "1.2",
			want:    tls.VersionTLS12,
			wantErr: false,
		},
		{
			name:    "TLS 1.3",
			version: "1.3",
			want:    tls.VersionTLS13,
			wantErr: false,
		},
		{
			name:    "Empty defaults to 1.3",
			version: "",
			want:    tls.VersionTLS13,
			wantErr: false,
		},
		{
			name:    "Unsupported TLS 1.1",
			version: "1.1",
			want:    0,
			wantErr: true,
		},
		{
			name:    "Unsupported TLS 1.0",
			version: "1.0",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTLSVersion(tt.version)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestBuildTLSConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		check   func(t *testing.T, tlsConfig *tls.Config)
	}{
		{
			name: "Default secure config",
			config: &Config{
				Enabled:    true,
				MinVersion: "1.3",
			},
			wantErr: false,
			check: func(t *testing.T, tlsConfig *tls.Config) {
				assert.Equal(t, uint16(tls.VersionTLS13), tlsConfig.MinVersion)
				assert.Equal(t, tls.RenegotiateNever, tlsConfig.Renegotiation)
			},
		},
		{
			name: "TLS 1.2 with cipher suites",
			config: &Config{
				Enabled:    true,
				MinVersion: "1.2",
			},
			wantErr: false,
			check: func(t *testing.T, tlsConfig *tls.Config) {
				assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)
				assert.NotEmpty(t, tlsConfig.CipherSuites)
				// PreferServerCipherSuites is deprecated in Go 1.24+
			},
		},
		{
			name: "Disabled TLS",
			config: &Config{
				Enabled: false,
			},
			wantErr: false,
			check: func(t *testing.T, tlsConfig *tls.Config) {
				assert.Nil(t, tlsConfig)
			},
		},
		{
			name: "Invalid min version",
			config: &Config{
				Enabled:    true,
				MinVersion: "1.1",
			},
			wantErr: true,
		},
		{
			name: "With session tickets",
			config: &Config{
				Enabled:        true,
				MinVersion:     "1.3",
				SessionTickets: true,
			},
			wantErr: false,
			check: func(t *testing.T, tlsConfig *tls.Config) {
				assert.False(t, tlsConfig.SessionTicketsDisabled) // Inverted logic
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsConfig, err := tt.config.BuildTLSConfig()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.check != nil {
					tt.check(t, tlsConfig)
				}
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		isProduction bool
		wantErr      bool
		errorMsg     string
	}{
		{
			name: "Valid production config",
			config: &Config{
				Enabled:            true,
				MinVersion:         "1.3",
				VerifyCertificates: true,
				InsecureSkipVerify: false,
			},
			isProduction: true,
			wantErr:      false,
		},
		{
			name: "Production with insecure skip verify",
			config: &Config{
				Enabled:            true,
				MinVersion:         "1.3",
				InsecureSkipVerify: true,
			},
			isProduction: true,
			wantErr:      true,
			errorMsg:     "insecure_skip_verify cannot be true in production",
		},
		{
			name: "Production without certificate verification",
			config: &Config{
				Enabled:            true,
				MinVersion:         "1.3",
				VerifyCertificates: false,
			},
			isProduction: true,
			wantErr:      true,
			errorMsg:     "verify_certificates must be true in production",
		},
		{
			name: "Development with relaxed security",
			config: &Config{
				Enabled:            true,
				MinVersion:         "1.2",
				InsecureSkipVerify: true,
			},
			isProduction: false,
			wantErr:      false,
		},
		{
			name: "Below minimum version",
			config: &Config{
				Enabled:    true,
				MinVersion: "1.1",
			},
			isProduction: false,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config, tt.isProduction)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGetHardwareAcceleration(t *testing.T) {
	info := GetHardwareAcceleration()
	assert.NotNil(t, info)
	
	// Just verify the function returns a map with expected keys
	// The actual values depend on the hardware
	t.Logf("Hardware acceleration info: %+v", info)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	assert.True(t, cfg.Enabled)
	assert.Equal(t, DefaultMinVersion, cfg.MinVersion)
	assert.True(t, cfg.VerifyCertificates)
	assert.True(t, cfg.SessionTickets)
	assert.Equal(t, 1000, cfg.SessionCacheSize)
}

func TestParseClientAuthType(t *testing.T) {
	tests := []struct {
		name     string
		authType string
		want     tls.ClientAuthType
		wantErr  bool
	}{
		{"None", "none", tls.NoClientCert, false},
		{"Request", "request", tls.RequestClientCert, false},
		{"Require", "require", tls.RequireAnyClientCert, false},
		{"Verify", "verify", tls.RequireAndVerifyClientCert, false},
		{"Invalid", "invalid", tls.NoClientCert, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseClientAuthType(tt.authType)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}