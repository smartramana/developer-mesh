package services

import (
	"context"
	"sort"
	"sync"

	"github.com/S-Corkum/devops-mcp/pkg/models"
	"github.com/S-Corkum/devops-mcp/pkg/rules"
)

// AssignmentEngine handles intelligent task assignment
type AssignmentEngine struct {
	ruleEngine rules.Engine
	strategies map[string]AssignmentStrategy
	scoreCache sync.Map // map[string]float64 - agent scores

	// Metrics
	assignments  map[string]int
	assignmentMu sync.RWMutex
}

// NewAssignmentEngine creates a new assignment engine
func NewAssignmentEngine(ruleEngine rules.Engine) *AssignmentEngine {
	e := &AssignmentEngine{
		ruleEngine:  ruleEngine,
		strategies:  make(map[string]AssignmentStrategy),
		assignments: make(map[string]int),
	}

	// Register default strategies
	e.RegisterStrategy("round_robin", &RoundRobinStrategy{})
	e.RegisterStrategy("least_loaded", &LeastLoadedStrategy{})
	e.RegisterStrategy("capability_match", &CapabilityMatchStrategy{})
	e.RegisterStrategy("performance_based", &PerformanceBasedStrategy{})
	e.RegisterStrategy("cost_optimized", &CostOptimizedStrategy{})

	return e
}

// RegisterStrategy registers an assignment strategy
func (e *AssignmentEngine) RegisterStrategy(name string, strategy AssignmentStrategy) {
	e.strategies[name] = strategy
}

// FindBestAgent finds the best agent for a task using rules and strategies
func (e *AssignmentEngine) FindBestAgent(ctx context.Context, task *models.Task) (*models.Agent, error) {
	// Get assignment rules if rule engine is configured
	var assignmentRules []rules.Rule
	if e.ruleEngine != nil {
		var err error
		assignmentRules, err = e.ruleEngine.GetRules(ctx, "task.assignment", map[string]interface{}{
			"task_type": task.Type,
			"priority":  task.Priority,
		})
		if err != nil {
			return nil, err
		}
	}

	// Get eligible agents
	eligibleAgents, err := e.getEligibleAgents(ctx, task, assignmentRules)
	if err != nil {
		return nil, err
	}

	if len(eligibleAgents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	// Determine strategy
	strategy := e.determineStrategy(ctx, task, assignmentRules)

	// Apply strategy
	return strategy.Assign(ctx, task, eligibleAgents)
}

func (e *AssignmentEngine) getEligibleAgents(ctx context.Context, task *models.Task, rules []rules.Rule) ([]*models.Agent, error) {
	// TODO: Get all available agents from agent service
	// For now, return empty slice
	return []*models.Agent{}, nil
}

func (e *AssignmentEngine) determineStrategy(ctx context.Context, task *models.Task, rules []rules.Rule) AssignmentStrategy {
	// TODO: Determine strategy based on task type and rules
	// For now, return capability match strategy
	return e.strategies["capability_match"]
}

// RoundRobinStrategy assigns tasks in round-robin fashion
type RoundRobinStrategy struct {
	counter uint64
	mu      sync.Mutex
}

func (s *RoundRobinStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	s.mu.Lock()
	index := s.counter % uint64(len(agents))
	s.counter++
	s.mu.Unlock()

	return agents[index], nil
}

func (s *RoundRobinStrategy) GetName() string {
	return "round_robin"
}

// LeastLoadedStrategy assigns tasks to the least loaded agent
type LeastLoadedStrategy struct{}

func (s *LeastLoadedStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	// TODO: Get workload for each agent and select the least loaded
	// For now, return the first agent
	return agents[0], nil
}

func (s *LeastLoadedStrategy) GetName() string {
	return "least_loaded"
}

// CapabilityMatchStrategy assigns tasks based on agent capabilities
type CapabilityMatchStrategy struct{}

func (s *CapabilityMatchStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	// Extract required capabilities
	requiredCaps := extractRequiredCapabilities(task)

	// Score agents based on capability match
	type agentScore struct {
		agent *models.Agent
		score float64
	}

	scores := make([]agentScore, 0, len(agents))

	for _, agent := range agents {
		score := calculateCapabilityScore(agent.Capabilities, requiredCaps)
		scores = append(scores, agentScore{agent: agent, score: score})
	}

	// Sort by score
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if len(scores) > 0 && scores[0].score > 0 {
		return scores[0].agent, nil
	}

	return nil, ErrNoCapableAgent
}

func (s *CapabilityMatchStrategy) GetName() string {
	return "capability_match"
}

// PerformanceBasedStrategy assigns tasks based on agent performance metrics
type PerformanceBasedStrategy struct {
	performanceCache sync.Map
}

func (s *PerformanceBasedStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	var bestAgent *models.Agent
	bestScore := 0.0

	for _, agent := range agents {
		// Get cached performance or calculate
		var perf models.AgentPerformance
		if cached, ok := s.performanceCache.Load(agent.ID); ok {
			perf = cached.(models.AgentPerformance)
		} else {
			// Calculate performance metrics
			perf = calculateAgentPerformance(ctx, agent.ID, task.Type)
			s.performanceCache.Store(agent.ID, perf)
		}

		// Calculate score based on:
		// - Success rate
		// - Average completion time
		// - Current load
		score := perf.SuccessRate*0.4 +
			(1.0-perf.LoadFactor)*0.3 +
			perf.SpeedScore*0.3

		if score > bestScore {
			bestScore = score
			bestAgent = agent
		}
	}

	if bestAgent == nil {
		return nil, ErrNoCapableAgent
	}

	return bestAgent, nil
}

func (s *PerformanceBasedStrategy) GetName() string {
	return "performance_based"
}

// CostOptimizedStrategy assigns tasks based on cost optimization
type CostOptimizedStrategy struct{}

func (s *CostOptimizedStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	// TODO: Implement cost-based assignment
	// For now, return the first agent
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}
	return agents[0], nil
}

func (s *CostOptimizedStrategy) GetName() string {
	return "cost_optimized"
}

// Helper functions

func extractRequiredCapabilities(task *models.Task) []string {
	// TODO: Extract capabilities from task parameters
	caps := []string{}
	if task.Type != "" {
		caps = append(caps, task.Type)
	}
	return caps
}

func calculateCapabilityScore(agentCaps []string, requiredCaps []string) float64 {
	if len(requiredCaps) == 0 {
		return 1.0
	}

	// Create a set of agent capabilities
	capSet := make(map[string]bool)
	for _, cap := range agentCaps {
		capSet[cap] = true
	}

	// Calculate match percentage
	matches := 0
	for _, req := range requiredCaps {
		if capSet[req] {
			matches++
		}
	}

	return float64(matches) / float64(len(requiredCaps))
}

func calculateAgentPerformance(ctx context.Context, agentID string, taskType string) models.AgentPerformance {
	// TODO: Calculate actual performance metrics
	return models.AgentPerformance{
		SuccessRate: 0.95,
		LoadFactor:  0.5,
		SpeedScore:  0.8,
	}
}