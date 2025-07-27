package adapters

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/tools"
)

// DiscoveryPattern represents a learned pattern for API discovery
type DiscoveryPattern struct {
	Domain          string    `json:"domain"`
	SuccessfulPaths []string  `json:"successful_paths"`
	AuthMethod      string    `json:"auth_method"`
	APIFormat       string    `json:"api_format"`
	LastUpdated     time.Time `json:"last_updated"`
	SuccessCount    int       `json:"success_count"`
}

// LearningDiscoveryService learns from successful discoveries
type LearningDiscoveryService struct {
	patterns map[string]*DiscoveryPattern
	mu       sync.RWMutex
	store    DiscoveryPatternStore
}

// DiscoveryPatternStore interface for persisting learned patterns
type DiscoveryPatternStore interface {
	SavePattern(pattern *DiscoveryPattern) error
	LoadPatterns() (map[string]*DiscoveryPattern, error)
	GetPatternByDomain(domain string) (*DiscoveryPattern, error)
}

// NewLearningDiscoveryService creates a new learning discovery service
func NewLearningDiscoveryService(store DiscoveryPatternStore) *LearningDiscoveryService {
	patterns, _ := store.LoadPatterns()
	if patterns == nil {
		patterns = make(map[string]*DiscoveryPattern)
	}

	return &LearningDiscoveryService{
		patterns: patterns,
		store:    store,
	}
}

// LearnFromSuccess records a successful discovery pattern
func (l *LearningDiscoveryService) LearnFromSuccess(config tools.ToolConfig, result *tools.DiscoveryResult) error {
	if result.Status != tools.DiscoveryStatusSuccess {
		return nil
	}

	domain := l.extractDomain(config.BaseURL)
	if domain == "" {
		return fmt.Errorf("invalid base URL")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	pattern, exists := l.patterns[domain]
	if !exists {
		pattern = &DiscoveryPattern{
			Domain:          domain,
			SuccessfulPaths: []string{},
			LastUpdated:     time.Now(),
		}
		l.patterns[domain] = pattern
	}

	// Update pattern with successful discovery
	if result.SpecURL != "" {
		path := l.extractPath(result.SpecURL, config.BaseURL)
		if path != "" && !l.containsPath(pattern.SuccessfulPaths, path) {
			pattern.SuccessfulPaths = append(pattern.SuccessfulPaths, path)
		}
	}

	// Learn authentication method
	if authMethod, ok := result.Metadata["auth_method"].(string); ok {
		pattern.AuthMethod = authMethod
	}

	// Learn API format
	if apiFormat, ok := result.Metadata["api_format"].(string); ok {
		pattern.APIFormat = apiFormat
	}

	pattern.SuccessCount++
	pattern.LastUpdated = time.Now()

	// Persist the pattern
	return l.store.SavePattern(pattern)
}

// GetSuggestedPaths returns suggested paths based on learned patterns
func (l *LearningDiscoveryService) GetSuggestedPaths(baseURL string) []string {
	if l == nil {
		return nil
	}

	domain := l.extractDomain(baseURL)
	if domain == "" {
		return nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	// Direct domain match
	if pattern, exists := l.patterns[domain]; exists && pattern != nil {
		return pattern.SuccessfulPaths
	}

	// Try to find similar domains
	var suggestedPaths []string
	for d, pattern := range l.patterns {
		if pattern != nil && l.areSimilarDomains(domain, d) {
			suggestedPaths = append(suggestedPaths, pattern.SuccessfulPaths...)
		}
	}

	return l.deduplicatePaths(suggestedPaths)
}

// GetLearnedAuthMethod returns learned authentication method for a domain
func (l *LearningDiscoveryService) GetLearnedAuthMethod(baseURL string) string {
	if l == nil {
		return ""
	}

	domain := l.extractDomain(baseURL)
	if domain == "" {
		return ""
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	if pattern, exists := l.patterns[domain]; exists && pattern != nil {
		return pattern.AuthMethod
	}

	return ""
}

// extractDomain extracts the domain from a URL
func (l *LearningDiscoveryService) extractDomain(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return u.Host
}

// extractPath extracts the path portion from a full URL
func (l *LearningDiscoveryService) extractPath(fullURL, baseURL string) string {
	if strings.HasPrefix(fullURL, baseURL) {
		return strings.TrimPrefix(fullURL, baseURL)
	}

	// Try to parse the full URL
	u, err := url.Parse(fullURL)
	if err != nil {
		// If it doesn't parse as a URL, return as-is
		return fullURL
	}

	return u.Path
}

// containsPath checks if a path exists in the list
func (l *LearningDiscoveryService) containsPath(paths []string, path string) bool {
	for _, p := range paths {
		if p == path {
			return true
		}
	}
	return false
}

// areSimilarDomains checks if two domains are similar
func (l *LearningDiscoveryService) areSimilarDomains(domain1, domain2 string) bool {
	// Remove common subdomains
	domain1 = strings.TrimPrefix(domain1, "api.")
	domain1 = strings.TrimPrefix(domain1, "www.")
	domain2 = strings.TrimPrefix(domain2, "api.")
	domain2 = strings.TrimPrefix(domain2, "www.")

	// Check if they share the same base domain
	parts1 := strings.Split(domain1, ".")
	parts2 := strings.Split(domain2, ".")

	if len(parts1) >= 2 && len(parts2) >= 2 {
		// Compare last two parts (e.g., "example.com")
		base1 := parts1[len(parts1)-2] + "." + parts1[len(parts1)-1]
		base2 := parts2[len(parts2)-2] + "." + parts2[len(parts2)-1]
		return base1 == base2
	}

	return false
}

// deduplicatePaths removes duplicate paths
func (l *LearningDiscoveryService) deduplicatePaths(paths []string) []string {
	seen := make(map[string]bool)
	var unique []string

	for _, path := range paths {
		if !seen[path] {
			seen[path] = true
			unique = append(unique, path)
		}
	}

	return unique
}

// GetPopularPatterns returns the most successful discovery patterns
func (l *LearningDiscoveryService) GetPopularPatterns() []DiscoveryPattern {
	if l == nil {
		return nil
	}

	l.mu.RLock()
	defer l.mu.RUnlock()

	var patterns []DiscoveryPattern
	for _, pattern := range l.patterns {
		if pattern != nil {
			patterns = append(patterns, *pattern)
		}
	}

	// Sort by success count (you'd implement proper sorting)
	// For now, just return all patterns
	return patterns
}

// InMemoryPatternStore is a simple in-memory implementation of DiscoveryPatternStore
type InMemoryPatternStore struct {
	patterns map[string]*DiscoveryPattern
	mu       sync.RWMutex
}

// NewInMemoryPatternStore creates a new in-memory pattern store
func NewInMemoryPatternStore() *InMemoryPatternStore {
	return &InMemoryPatternStore{
		patterns: make(map[string]*DiscoveryPattern),
	}
}

// SavePattern saves a discovery pattern
func (s *InMemoryPatternStore) SavePattern(pattern *DiscoveryPattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.patterns[pattern.Domain] = pattern
	return nil
}

// LoadPatterns loads all patterns
func (s *InMemoryPatternStore) LoadPatterns() (map[string]*DiscoveryPattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy
	patterns := make(map[string]*DiscoveryPattern)
	for k, v := range s.patterns {
		patterns[k] = v
	}

	return patterns, nil
}

// GetPatternByDomain gets a pattern by domain
func (s *InMemoryPatternStore) GetPatternByDomain(domain string) (*DiscoveryPattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	pattern, exists := s.patterns[domain]
	if !exists {
		return nil, fmt.Errorf("pattern not found for domain: %s", domain)
	}

	return pattern, nil
}
