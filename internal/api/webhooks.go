package api

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/internal/interfaces"
	"github.com/rs/zerolog/log"
)

// GitHubWebhookHandler creates an HTTP handler for GitHub webhook events
// It validates the webhook signature and processes the payload
func GitHubWebhookHandler(config interfaces.WebhookConfigInterface) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract and validate GitHub event type
		eventType := r.Header.Get("X-GitHub-Event")
		if eventType == "" {
			http.Error(w, "Missing X-GitHub-Event header", http.StatusBadRequest)
			log.Warn().Msg("GitHub webhook missing event header")
			return
		}

		// Verify HTTP method
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			log.Warn().Str("method", r.Method).Msg("GitHub webhook received non-POST request")
			return
		}

		// Check if event type is allowed
		allowedEvents := config.GitHubAllowedEvents()
		isEventAllowed := false
		for _, allowed := range allowedEvents {
			if eventType == allowed {
				isEventAllowed = true
				break
			}
		}

		if !isEventAllowed {
			http.Error(w, "Event type not allowed", http.StatusForbidden)
			log.Warn().Str("eventType", eventType).Msg("GitHub webhook event type not allowed")
			return
		}

		// Read request body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			log.Error().Err(err).Msg("GitHub webhook failed to read request body")
			return
		}
		// Restore the body for later use
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Verify signature
		signature := r.Header.Get("X-Hub-Signature-256")
		if signature == "" {
			http.Error(w, "Missing X-Hub-Signature-256 header", http.StatusUnauthorized)
			log.Warn().Msg("GitHub webhook missing signature header")
			return
		}

		if !verifySignature(bodyBytes, signature, config.GitHubSecret()) {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			log.Warn().Msg("GitHub webhook invalid signature")
			return
		}

		// Parse payload
		var payload map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			http.Error(w, "Failed to parse payload", http.StatusBadRequest)
			log.Error().Err(err).Msg("GitHub webhook failed to parse payload")
			return
		}

		// Extract repository and sender information for logging
		repoName := extractRepoName(payload)
		senderName := extractSenderName(payload)

		// Log the event
		log.Info().Str("eventType", eventType).Str("repository", repoName).Str("sender", senderName).Msg("GitHub webhook received")

		// TODO: Process the event based on type
		// This can be extended to handle different event types with custom handlers

		// Return success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Webhook received successfully"))
	}
}

// verifySignature verifies the GitHub webhook signature
func verifySignature(payload []byte, signature, secret string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	// Extract the hash from the signature header
	signatureHash := strings.TrimPrefix(signature, "sha256=")

	// Compute the HMAC hash
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedHash := hex.EncodeToString(mac.Sum(nil))

	// Compare the computed hash with the provided hash
	return hmac.Equal([]byte(signatureHash), []byte(expectedHash))
}

// extractRepoName extracts the repository name from the payload
func extractRepoName(payload map[string]interface{}) string {
	if repo, ok := payload["repository"].(map[string]interface{}); ok {
		if fullName, ok := repo["full_name"].(string); ok {
			return fullName
		}
	}
	return "unknown"
}

// extractSenderName extracts the sender name from the payload
func extractSenderName(payload map[string]interface{}) string {
	if sender, ok := payload["sender"].(map[string]interface{}); ok {
		if login, ok := sender["login"].(string); ok {
			return login
		}
	}
	return "unknown"
}

// GitHubIPValidator validates if a request comes from GitHub IP range
type GitHubIPValidator struct {
	ipRanges    []net.IPNet
	ipRangesIPv6 []net.IPNet
	lastUpdated time.Time
	cacheTTL    time.Duration
	mutex       sync.RWMutex
}

// GitHubMeta represents the GitHub meta API response
type GitHubMeta struct {
	Hooks      []string `json:"hooks"`
	HooksIPv4  []string `json:"hooks_ipv4"`
	HooksIPv6  []string `json:"hooks_ipv6"`
	Web        []string `json:"web"`
	API        []string `json:"api"`
	Git        []string `json:"git"`
	Packages   []string `json:"packages"`
	Pages      []string `json:"pages"`
	Importer   []string `json:"importer"`
	Actions    []string `json:"actions"`
	Dependabot []string `json:"dependabot"`
}

// NewGitHubIPValidator creates a new GitHub IP validator
func NewGitHubIPValidator() *GitHubIPValidator {
	return &GitHubIPValidator{
		cacheTTL: 24 * time.Hour, // Update IP ranges every 24 hours
	}
}

// FetchGitHubIPRanges fetches and caches GitHub's IP ranges
func (v *GitHubIPValidator) FetchGitHubIPRanges() error {
	v.mutex.Lock()
	defer v.mutex.Unlock()

	// Check if cache is still valid
	if !v.lastUpdated.IsZero() && time.Since(v.lastUpdated) < v.cacheTTL {
		return nil
	}

	// Fetch the GitHub meta API
	resp, err := http.Get("https://api.github.com/meta")
	if err != nil {
		return fmt.Errorf("failed to fetch GitHub meta API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub meta API returned non-OK status: %d", resp.StatusCode)
	}

	// Parse the response
	var meta GitHubMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return fmt.Errorf("failed to parse GitHub meta API response: %w", err)
	}

	// Parse IPv4 ranges
	v.ipRanges = make([]net.IPNet, 0, len(meta.Hooks))
	for _, cidr := range meta.Hooks {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			log.Warn().Str("cidr", cidr).Err(err).Msg("Failed to parse GitHub IP range")
			continue
		}
		v.ipRanges = append(v.ipRanges, *ipNet)
	}

	// Parse IPv6 ranges
	v.ipRangesIPv6 = make([]net.IPNet, 0, len(meta.HooksIPv6))
	for _, cidr := range meta.HooksIPv6 {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			log.Warn().Str("cidr", cidr).Err(err).Msg("Failed to parse GitHub IPv6 range")
			continue
		}
		v.ipRangesIPv6 = append(v.ipRangesIPv6, *ipNet)
	}

	v.lastUpdated = time.Now()
	log.Info().Int("ipv4Count", len(v.ipRanges)).Int("ipv6Count", len(v.ipRangesIPv6)).Msg("Updated GitHub IP ranges")
	return nil
}

// IsGitHubIP checks if an IP address belongs to GitHub's IP ranges
func (v *GitHubIPValidator) IsGitHubIP(ipStr string) bool {
	// Ensure we have the latest IP ranges
	if err := v.FetchGitHubIPRanges(); err != nil {
		log.Error().Err(err).Msg("Failed to fetch GitHub IP ranges")
		// If we can't fetch IP ranges, we're conservative and assume the IP is not from GitHub
		return false
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		log.Warn().Str("ip", ipStr).Msg("Invalid IP address format")
		return false
	}

	v.mutex.RLock()
	defer v.mutex.RUnlock()

	// Check if the IP is in GitHub's ranges
	if ip.To4() != nil {
		// IPv4
		for _, ipNet := range v.ipRanges {
			if ipNet.Contains(ip) {
				return true
			}
		}
	} else {
		// IPv6
		for _, ipNet := range v.ipRangesIPv6 {
			if ipNet.Contains(ip) {
				return true
			}
		}
	}

	return false
}

// GitHubIPValidationMiddleware creates middleware that validates if requests come from GitHub IP ranges
func GitHubIPValidationMiddleware(validator *GitHubIPValidator, config interfaces.WebhookConfigInterface) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip IP validation if disabled in config
			if !config.GitHubIPValidationEnabled() {
				next.ServeHTTP(w, r)
				return
			}

			// Get the client IP
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				// If we can't parse the remote address, assume it's the whole string
				ip = r.RemoteAddr
			}

			// Check if the IP is from GitHub
			if !validator.IsGitHubIP(ip) {
				http.Error(w, "Forbidden: IP not authorized", http.StatusForbidden)
				log.Warn().Str("ip", ip).Msg("Blocked request from non-GitHub IP")
				return
			}

			// IP is authorized, proceed to the next handler
			next.ServeHTTP(w, r)
		})
	}
}