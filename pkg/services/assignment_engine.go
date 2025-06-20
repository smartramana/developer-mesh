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
	ruleEngine   rules.Engine
	agentService AgentService
	strategies   map[string]AssignmentStrategy

	// Metrics
	assignments map[string]int
}

// NewAssignmentEngine creates a new assignment engine
func NewAssignmentEngine(ruleEngine rules.Engine, agentService AgentService) *AssignmentEngine {
	e := &AssignmentEngine{
		ruleEngine:   ruleEngine,
		agentService: agentService,
		strategies:   make(map[string]AssignmentStrategy),
		assignments:  make(map[string]int),
	}

	// Register default strategies
	e.RegisterStrategy("round_robin", &RoundRobinStrategy{})
	e.RegisterStrategy("least_loaded", &LeastLoadedStrategy{agentService: agentService})
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
	// Get all available agents
	agents, err := e.agentService.GetAvailableAgents(ctx)
	if err != nil {
		return nil, err
	}

	// Filter for agents that match the task tenant and can handle the task type
	var eligibleAgents []*models.Agent
	for _, agent := range agents {
		// Check tenant match
		if agent.TenantID != task.TenantID {
			continue
		}

		// Check if agent is active
		if agent.Status != string(models.AgentStatusActive) {
			continue
		}

		// Get agent capabilities
		capabilities, err := e.agentService.GetAgentCapabilities(ctx, agent.ID)
		if err != nil {
			continue // Skip agents whose capabilities can't be retrieved
		}

		// Check if agent can handle this task type
		canHandle := false
		for _, capability := range capabilities {
			if capability == string(task.Type) || capability == "all" {
				canHandle = true
				break
			}
		}

		if canHandle {
			eligibleAgents = append(eligibleAgents, agent)
		}
	}

	// Apply additional rule-based filtering
	if e.ruleEngine != nil && len(rules) > 0 && len(eligibleAgents) > 0 {
		filtered := make([]*models.Agent, 0, len(eligibleAgents))
		for _, agent := range eligibleAgents {
			eligible := true
			for _, rule := range rules {
				// Evaluate rule against agent and task context
				decision, err := e.ruleEngine.Evaluate(ctx, rule.ID.String(), map[string]interface{}{
					"agent": agent,
					"task":  task,
				})
				if err != nil || decision == nil || !decision.Allowed {
					eligible = false
					break
				}
			}
			if eligible {
				filtered = append(filtered, agent)
			}
		}
		eligibleAgents = filtered
	}

	return eligibleAgents, nil
}

func (e *AssignmentEngine) determineStrategy(ctx context.Context, task *models.Task, rules []rules.Rule) AssignmentStrategy {
	// Check rules for strategy preference
	if e.ruleEngine != nil && len(rules) > 0 {
		for _, rule := range rules {
			// Check if rule relates to assignment strategy
			if rule.Name == "assignment_strategy" || rule.Category == "assignment" {
				// Evaluate rule to get strategy preference
				decision, err := e.ruleEngine.Evaluate(ctx, rule.ID.String(), map[string]interface{}{
					"task": task,
				})
				if err == nil && decision != nil && decision.Allowed {
					// Try to extract strategy from decision metadata
					if decision.Metadata != nil {
						if strategyName, ok := decision.Metadata["strategy"].(string); ok {
							if strategy, exists := e.strategies[strategyName]; exists {
								return strategy
							}
						}
					}
				}
			}
		}
	}

	// Default strategy based on task type
	switch task.Type {
	case "build", "deploy":
		// For builds and deploys, use least loaded to distribute work
		return e.strategies["least_loaded"]
	case "test":
		// For tests, use capability match to ensure proper environment
		return e.strategies["capability_match"]
	case "monitor", "health_check":
		// For monitoring, use round robin for even distribution
		return e.strategies["round_robin"]
	case "backup", "restore":
		// For backups, use performance based to complete quickly
		return e.strategies["performance_based"]
	default:
		// Default to capability match for safety
		return e.strategies["capability_match"]
	}
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
type LeastLoadedStrategy struct {
	agentService AgentService
}

func (s *LeastLoadedStrategy) Assign(ctx context.Context, task *models.Task, agents []*models.Agent) (*models.Agent, error) {
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	// Get workload for each agent and select the least loaded
	var leastLoadedAgent *models.Agent
	minWorkload := -1

	for _, agent := range agents {
		workload, err := s.agentService.GetAgentWorkload(ctx, agent.ID)
		if err != nil {
			// If we can't get workload, skip this agent
			continue
		}

		// Calculate total workload (active + queued tasks)
		totalWorkload := workload.ActiveTasks + workload.QueuedTasks

		if minWorkload == -1 || totalWorkload < minWorkload {
			minWorkload = totalWorkload
			leastLoadedAgent = agent
		}
	}

	if leastLoadedAgent == nil {
		// If we couldn't get workload for any agent, fall back to first agent
		return agents[0], nil
	}

	return leastLoadedAgent, nil
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
	if len(agents) == 0 {
		return nil, ErrNoEligibleAgents
	}

	// Calculate cost for each agent based on their rate and estimated task duration
	type agentCost struct {
		agent *models.Agent
		cost  float64
	}

	costs := make([]agentCost, 0, len(agents))
	for _, agent := range agents {
		// Get agent cost per hour from metadata
		hourlyRate := 10.0 // Default rate
		if agent.Metadata != nil {
			if rate, ok := agent.Metadata["hourly_rate"].(float64); ok {
				hourlyRate = rate
			}
		}

		// Estimate task duration based on agent performance metrics
		estimatedHours := 1.0 // Default 1 hour
		if agent.Metadata != nil {
			if avgTime, ok := agent.Metadata["avg_task_time"].(float64); ok {
				estimatedHours = avgTime / 3600 // Convert seconds to hours
			}
		}

		// Calculate total cost
		totalCost := hourlyRate * estimatedHours

		// Add overhead costs if any
		if agent.Metadata != nil {
			if overhead, ok := agent.Metadata["overhead_cost"].(float64); ok {
				totalCost += overhead
			}
		}

		costs = append(costs, agentCost{
			agent: agent,
			cost:  totalCost,
		})
	}

	// Sort by cost and select the cheapest
	sort.Slice(costs, func(i, j int) bool {
		return costs[i].cost < costs[j].cost
	})

	return costs[0].agent, nil
}

func (s *CostOptimizedStrategy) GetName() string {
	return "cost_optimized"
}

// Helper functions

func extractRequiredCapabilities(task *models.Task) []string {
	caps := []string{}

	// Always require the task type capability
	if task.Type != "" {
		caps = append(caps, task.Type)
	}

	// Extract capabilities from task parameters
	if task.Parameters != nil {
		// Check for required_capabilities parameter
		if reqCaps, ok := task.Parameters["required_capabilities"].([]interface{}); ok {
			for _, cap := range reqCaps {
				if capStr, ok := cap.(string); ok {
					caps = append(caps, capStr)
				}
			}
		}

		// Check for specific capability requirements
		if lang, ok := task.Parameters["language"].(string); ok {
			caps = append(caps, "lang:"+lang)
		}
		if env, ok := task.Parameters["environment"].(string); ok {
			caps = append(caps, "env:"+env)
		}
		if tool, ok := task.Parameters["tool"].(string); ok {
			caps = append(caps, "tool:"+tool)
		}
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
	// In a production system, this would query historical performance data
	// from a metrics repository. For now, we calculate based on some heuristics.

	// Default performance metrics
	perf := models.AgentPerformance{
		AgentID:               agentID,
		TasksCompleted:        100,  // Assume 100 tasks completed
		TasksFailed:           5,    // 5 failures
		AverageCompletionTime: 300,  // 5 minutes average
		SuccessRate:           0.95, // 95% success rate
		LoadFactor:            0.5,  // 50% loaded
		SpeedScore:            0.8,  // 80% speed efficiency
		TaskTypeMetrics:       make(map[string]models.TaskMetrics),
	}

	// Adjust based on task type
	switch taskType {
	case "build":
		perf.AverageCompletionTime = 600 // Builds take longer
		perf.SpeedScore = 0.7
	case "test":
		perf.AverageCompletionTime = 300
		perf.SpeedScore = 0.85
	case "deploy":
		perf.AverageCompletionTime = 180
		perf.SuccessRate = 0.98 // Deploys should be highly reliable
	case "monitor":
		perf.AverageCompletionTime = 60
		perf.SpeedScore = 0.95 // Monitoring should be fast
	}

	// In production, we would also:
	// 1. Query recent task completion rates for this agent
	// 2. Calculate actual average completion times
	// 3. Check current workload from agent service
	// 4. Factor in recent failures or timeouts

	return perf
}
