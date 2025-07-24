package services

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// IPValidator provides flexible IP validation for webhooks and other services
type IPValidator struct {
	allowedRanges map[string][]net.IPNet // Key is source name (e.g., "github", "custom")
	mu            sync.RWMutex
	logger        observability.Logger
}

// NewIPValidator creates a new IP validator
func NewIPValidator(logger observability.Logger) *IPValidator {
	if logger == nil {
		logger = observability.NewLogger("ip-validator")
	}

	validator := &IPValidator{
		allowedRanges: make(map[string][]net.IPNet),
		logger:        logger,
	}

	// Initialize from environment variables
	validator.LoadFromEnvironment()

	return validator
}

// LoadFromEnvironment loads IP ranges from environment variables
// Supports:
// - IP_ALLOWED_RANGES_<SOURCE>=<cidr1>,<cidr2>,...
// - IP_ALLOWED_RANGES=<cidr1>,<cidr2>,... (global allowlist)
// Example:
// - IP_ALLOWED_RANGES_GITHUB=192.30.252.0/22,185.199.108.0/22
// - IP_ALLOWED_RANGES_OFFICE=10.0.0.0/8
// - IP_ALLOWED_RANGES=192.168.1.0/24,172.16.0.0/12
func (v *IPValidator) LoadFromEnvironment() {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Load global allowed ranges
	if globalRanges := os.Getenv("IP_ALLOWED_RANGES"); globalRanges != "" {
		v.loadRangesForSource("global", globalRanges)
	}

	// Load source-specific ranges
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "IP_ALLOWED_RANGES_") {
			parts := strings.SplitN(env, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]

				// Extract source name from key
				source := strings.ToLower(strings.TrimPrefix(key, "IP_ALLOWED_RANGES_"))
				v.loadRangesForSource(source, value)
			}
		}
	}

	// Log loaded configuration
	totalRanges := 0
	for source, ranges := range v.allowedRanges {
		totalRanges += len(ranges)
		v.logger.Info("Loaded IP ranges for source", map[string]any{
			"source": source,
			"count":  len(ranges),
		})
	}

	v.logger.Info("IP validator initialized", map[string]any{
		"sources":      len(v.allowedRanges),
		"total_ranges": totalRanges,
	})
}

// loadRangesForSource parses and loads CIDR ranges for a specific source
func (v *IPValidator) loadRangesForSource(source string, rangesStr string) {
	ranges := strings.Split(rangesStr, ",")
	var validRanges []net.IPNet

	for _, cidr := range ranges {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}

		// Parse CIDR notation
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			// Try parsing as a single IP
			ip := net.ParseIP(cidr)
			if ip == nil {
				v.logger.Warn("Invalid IP/CIDR format", map[string]any{
					"source": source,
					"value":  cidr,
					"error":  err.Error(),
				})
				continue
			}

			// Convert single IP to /32 or /128 CIDR
			if ip.To4() != nil {
				ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}
			} else {
				ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}
			}
		}

		validRanges = append(validRanges, *ipNet)
	}

	if len(validRanges) > 0 {
		v.allowedRanges[source] = validRanges
	}
}

// AddRange adds an IP range for a specific source
func (v *IPValidator) AddRange(source string, cidr string) error {
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR notation: %w", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	v.allowedRanges[source] = append(v.allowedRanges[source], *ipNet)

	v.logger.Info("Added IP range", map[string]any{
		"source": source,
		"cidr":   cidr,
	})

	return nil
}

// IsAllowed checks if an IP is allowed based on configured ranges
func (v *IPValidator) IsAllowed(ipStr string) bool {
	// If no ranges are configured, allow all (open mode)
	v.mu.RLock()
	defer v.mu.RUnlock()

	if len(v.allowedRanges) == 0 {
		v.logger.Debug("No IP ranges configured, allowing all", map[string]any{
			"ip": ipStr,
		})
		return true
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		v.logger.Warn("Invalid IP address format", map[string]any{
			"ip": ipStr,
		})
		return false
	}

	// Check all sources
	for source, ranges := range v.allowedRanges {
		for _, ipNet := range ranges {
			if ipNet.Contains(ip) {
				v.logger.Debug("IP allowed", map[string]any{
					"ip":     ipStr,
					"source": source,
					"range":  ipNet.String(),
				})
				return true
			}
		}
	}

	v.logger.Debug("IP not in allowed ranges", map[string]any{
		"ip": ipStr,
	})
	return false
}

// IsAllowedForSource checks if an IP is allowed for a specific source
func (v *IPValidator) IsAllowedForSource(ipStr string, source string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Check source-specific ranges
	if ranges, ok := v.allowedRanges[source]; ok {
		for _, ipNet := range ranges {
			if ipNet.Contains(ip) {
				return true
			}
		}
	}

	// Also check global ranges
	if ranges, ok := v.allowedRanges["global"]; ok {
		for _, ipNet := range ranges {
			if ipNet.Contains(ip) {
				return true
			}
		}
	}

	return false
}

// GetConfiguredSources returns a list of configured sources
func (v *IPValidator) GetConfiguredSources() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	sources := make([]string, 0, len(v.allowedRanges))
	for source := range v.allowedRanges {
		sources = append(sources, source)
	}
	return sources
}

// ClearRanges clears all configured IP ranges
func (v *IPValidator) ClearRanges() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.allowedRanges = make(map[string][]net.IPNet)
	v.logger.Info("Cleared all IP ranges", nil)
}
