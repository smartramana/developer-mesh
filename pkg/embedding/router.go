package embedding

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/S-Corkum/devops-mcp/pkg/agents"
	"github.com/S-Corkum/devops-mcp/pkg/embedding/providers"
)

// SmartRouter handles intelligent routing between providers
type SmartRouter struct {
	config          *RouterConfig
	providers       map[string]providers.Provider
	circuitBreakers map[string]*CircuitBreaker
	loadBalancer    *LoadBalancer
	costOptimizer   *CostOptimizer
	qualityTracker  *QualityTracker
	mu              sync.RWMutex
}

// RouterConfig configures the smart router
type RouterConfig struct {
	CircuitBreakerConfig CircuitBreakerConfig
	LoadBalancerConfig   LoadBalancerConfig
	CostOptimizerConfig  CostOptimizerConfig
	QualityConfig        QualityConfig
}

// DefaultRouterConfig returns default router configuration
func DefaultRouterConfig() *RouterConfig {
	return &RouterConfig{
		CircuitBreakerConfig: CircuitBreakerConfig{
			FailureThreshold:   5,
			SuccessThreshold:   2,
			Timeout:            30 * time.Second,
			HalfOpenMaxRequests: 3,
		},
		LoadBalancerConfig: LoadBalancerConfig{
			Strategy: "weighted_round_robin",
		},
		CostOptimizerConfig: CostOptimizerConfig{
			MaxCostPerRequest: 0.01,
		},
		QualityConfig: QualityConfig{
			MinQualityScore: 0.8,
		},
	}
}

// RoutingRequest represents a request for routing decision
type RoutingRequest struct {
	AgentConfig *agents.AgentConfig
	TaskType    agents.TaskType
	RequestID   string
}

// RoutingDecision represents the routing decision
type RoutingDecision struct {
	Candidates []ProviderCandidate
	Strategy   string
}

// ProviderCandidate represents a provider/model candidate
type ProviderCandidate struct {
	Provider string
	Model    string
	Score    float64
	Reasons  []string
}

// NewSmartRouter creates a new smart router
func NewSmartRouter(config *RouterConfig, providers map[string]providers.Provider) *SmartRouter {
	r := &SmartRouter{
		config:          config,
		providers:       providers,
		circuitBreakers: make(map[string]*CircuitBreaker),
		loadBalancer:    NewLoadBalancer(config.LoadBalancerConfig),
		costOptimizer:   NewCostOptimizer(config.CostOptimizerConfig),
		qualityTracker:  NewQualityTracker(config.QualityConfig),
	}

	// Initialize circuit breakers for each provider
	for name := range providers {
		r.circuitBreakers[name] = NewCircuitBreaker(config.CircuitBreakerConfig)
	}

	return r
}

// SelectProvider selects the best provider for the request
func (r *SmartRouter) SelectProvider(ctx context.Context, req *RoutingRequest) (*RoutingDecision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get model preferences for the task
	primary, fallback := req.AgentConfig.GetModelsForTask(req.TaskType)
	if len(primary) == 0 && len(fallback) == 0 {
		return nil, fmt.Errorf("no models configured for task type: %s", req.TaskType)
	}

	// Build list of all candidate models
	allModels := append(primary, fallback...)
	candidates := r.scoreCandidates(req, allModels)

	// Sort by score
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	// Filter out providers with open circuit breakers
	var availableCandidates []ProviderCandidate
	for _, candidate := range candidates {
		if r.circuitBreakers[candidate.Provider].CanRequest() {
			availableCandidates = append(availableCandidates, candidate)
		}
	}

	if len(availableCandidates) == 0 {
		return nil, fmt.Errorf("no available providers (all circuit breakers open)")
	}

	return &RoutingDecision{
		Candidates: availableCandidates,
		Strategy:   string(req.AgentConfig.EmbeddingStrategy),
	}, nil
}

// scoreCandidates scores each candidate based on multiple factors
func (r *SmartRouter) scoreCandidates(req *RoutingRequest, models []string) []ProviderCandidate {
	var candidates []ProviderCandidate

	for _, model := range models {
		// Find which provider supports this model
		for providerName, provider := range r.providers {
			supportedModels := provider.GetSupportedModels()
			
			for _, supported := range supportedModels {
				if supported.Name == model {
					score, reasons := r.scoreProviderModel(req, providerName, supported)
					candidates = append(candidates, ProviderCandidate{
						Provider: providerName,
						Model:    model,
						Score:    score,
						Reasons:  reasons,
					})
					break
				}
			}
		}
	}

	return candidates
}

// scoreProviderModel calculates a score for a specific provider/model combination
func (r *SmartRouter) scoreProviderModel(req *RoutingRequest, providerName string, model providers.ModelInfo) (float64, []string) {
	var score float64
	var reasons []string

	// Base score
	score = 50.0

	// Strategy-based scoring
	switch req.AgentConfig.EmbeddingStrategy {
	case agents.StrategyQuality:
		// Prioritize larger models and quality providers
		if model.Dimensions >= 3072 {
			score += 30
			reasons = append(reasons, "high-dimension model (+30)")
		} else if model.Dimensions >= 1536 {
			score += 20
			reasons = append(reasons, "standard-dimension model (+20)")
		}

		// Quality score from tracker
		qualityScore := r.qualityTracker.GetScore(providerName, model.Name)
		score += qualityScore * 20
		reasons = append(reasons, fmt.Sprintf("quality score: %.2f (+%.1f)", qualityScore, qualityScore*20))

	case agents.StrategyCost:
		// Prioritize cheaper models
		costScore := 1.0 - (model.CostPer1MTokens / 0.20) // Normalize to max $0.20
		score += costScore * 40
		reasons = append(reasons, fmt.Sprintf("cost efficiency: $%.3f/1M tokens (+%.1f)", model.CostPer1MTokens, costScore*40))

	case agents.StrategySpeed:
		// Prioritize based on current load and latency
		load := r.loadBalancer.GetLoad(providerName)
		latencyScore := math.Max(0, 1.0-(load/100.0)) * 30
		score += latencyScore
		reasons = append(reasons, fmt.Sprintf("load: %.1f%% (+%.1f)", load, latencyScore))

		// Smaller models are generally faster
		if model.Dimensions <= 1024 {
			score += 10
			reasons = append(reasons, "small model for speed (+10)")
		}

	case agents.StrategyBalanced:
		// Balance all factors
		qualityScore := r.qualityTracker.GetScore(providerName, model.Name)
		costScore := 1.0 - (model.CostPer1MTokens / 0.20)
		load := r.loadBalancer.GetLoad(providerName)
		latencyScore := math.Max(0, 1.0-(load/100.0))

		score += qualityScore * 10
		score += costScore * 10
		score += latencyScore * 10
		reasons = append(reasons, fmt.Sprintf("balanced: quality=%.2f, cost=%.2f, latency=%.2f (+%.1f)",
			qualityScore, costScore, latencyScore, qualityScore*10+costScore*10+latencyScore*10))
	}

	// Circuit breaker health bonus
	cbHealth := r.circuitBreakers[providerName].HealthScore()
	score += cbHealth * 10
	reasons = append(reasons, fmt.Sprintf("circuit breaker health: %.2f (+%.1f)", cbHealth, cbHealth*10))

	// Task type compatibility bonus
	for _, taskType := range model.SupportedTaskTypes {
		if taskType == string(req.TaskType) {
			score += 15
			reasons = append(reasons, "task type match (+15)")
			break
		}
	}

	// Constraint compliance
	constraints := req.AgentConfig.Constraints

	// Cost constraint
	if constraints.MaxCostPerMonthUSD > 0 {
		// Estimate monthly cost based on rate
		estimatedMonthlyCost := model.CostPer1MTokens * float64(constraints.RateLimits.TokensPerHour) * 24 * 30 / 1_000_000
		if estimatedMonthlyCost > constraints.MaxCostPerMonthUSD {
			score -= 50
			reasons = append(reasons, fmt.Sprintf("exceeds cost limit: $%.2f > $%.2f (-50)",
				estimatedMonthlyCost, constraints.MaxCostPerMonthUSD))
		}
	}

	return score, reasons
}

// RecordResult records the result of using a provider
func (r *SmartRouter) RecordResult(provider string, success bool, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if success {
		r.circuitBreakers[provider].RecordSuccess()
		r.loadBalancer.RecordLatency(provider, latency)
		r.qualityTracker.RecordSuccess(provider)
	} else {
		r.circuitBreakers[provider].RecordFailure()
		r.qualityTracker.RecordFailure(provider)
	}
}

// GetCircuitBreakerStatus returns the status of a provider's circuit breaker
func (r *SmartRouter) GetCircuitBreakerStatus(provider string) *CircuitBreakerStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if cb, ok := r.circuitBreakers[provider]; ok {
		return cb.Status()
	}
	return nil
}

// LoadBalancer tracks provider load
type LoadBalancer struct {
	config   LoadBalancerConfig
	loads    map[string]*ProviderLoad
	mu       sync.RWMutex
}

type LoadBalancerConfig struct {
	Strategy string
}

type ProviderLoad struct {
	CurrentRequests int
	AvgLatency      time.Duration
	LastUpdated     time.Time
}

func NewLoadBalancer(config LoadBalancerConfig) *LoadBalancer {
	return &LoadBalancer{
		config: config,
		loads:  make(map[string]*ProviderLoad),
	}
}

func (lb *LoadBalancer) GetLoad(provider string) float64 {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if load, ok := lb.loads[provider]; ok {
		// Simple load calculation based on current requests and latency
		return float64(load.CurrentRequests) * float64(load.AvgLatency.Milliseconds()) / 1000.0
	}
	return 0
}

func (lb *LoadBalancer) RecordLatency(provider string, latency time.Duration) {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if load, ok := lb.loads[provider]; ok {
		// Simple moving average
		load.AvgLatency = (load.AvgLatency + latency) / 2
		load.LastUpdated = time.Now()
	} else {
		lb.loads[provider] = &ProviderLoad{
			AvgLatency:  latency,
			LastUpdated: time.Now(),
		}
	}
}

// CostOptimizer tracks and optimizes costs
type CostOptimizer struct {
	config CostOptimizerConfig
}

type CostOptimizerConfig struct {
	MaxCostPerRequest float64
}

func NewCostOptimizer(config CostOptimizerConfig) *CostOptimizer {
	return &CostOptimizer{
		config: config,
	}
}

// QualityTracker tracks provider quality
type QualityTracker struct {
	config  QualityConfig
	scores  map[string]*QualityScore
	mu      sync.RWMutex
}

type QualityConfig struct {
	MinQualityScore float64
}

type QualityScore struct {
	SuccessCount int
	FailureCount int
	LastUpdated  time.Time
}

func NewQualityTracker(config QualityConfig) *QualityTracker {
	return &QualityTracker{
		config: config,
		scores: make(map[string]*QualityScore),
	}
}

func (qt *QualityTracker) GetScore(provider, model string) float64 {
	qt.mu.RLock()
	defer qt.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", provider, model)
	if score, ok := qt.scores[key]; ok {
		total := score.SuccessCount + score.FailureCount
		if total == 0 {
			return 1.0 // Default to perfect score
		}
		return float64(score.SuccessCount) / float64(total)
	}
	return 1.0 // Default to perfect score for new providers
}

func (qt *QualityTracker) RecordSuccess(provider string) {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	if score, ok := qt.scores[provider]; ok {
		score.SuccessCount++
		score.LastUpdated = time.Now()
	} else {
		qt.scores[provider] = &QualityScore{
			SuccessCount: 1,
			LastUpdated:  time.Now(),
		}
	}
}

func (qt *QualityTracker) RecordFailure(provider string) {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	if score, ok := qt.scores[provider]; ok {
		score.FailureCount++
		score.LastUpdated = time.Now()
	} else {
		qt.scores[provider] = &QualityScore{
			FailureCount: 1,
			LastUpdated:  time.Now(),
		}
	}
}