package resilience

import "time"

// Counts holds metrics used by the circuit breaker and other resilience components
type Counts struct {
	// Basic counts
	Requests             int       // Total requests
	Successes            int       // Successful requests
	Failures             int       // Failed requests
	ConsecutiveSuccesses int       // Consecutive successful requests
	ConsecutiveFailures  int       // Consecutive failed requests
	
	// Additional counts for advanced circuit breakers
	Timeout              int       // Timed out requests
	ShortCircuited       int       // Requests that were short-circuited
	Rejected             int       // Requests rejected due to circuit open
	
	// Timestamps for analysis
	LastSuccess          time.Time // Time of last successful request
	LastFailure          time.Time // Time of last failed request
	LastTimeout          time.Time // Time of last timeout
	
	// For external compatibility
	TotalSuccesses       uint32    // Total successful requests (uint32)
	TotalFailures        uint32    // Total failed requests (uint32)
}

// NewCounts creates a new Counts instance
func NewCounts() Counts {
	return Counts{}
}

// RecordSuccess records a successful request
func (c *Counts) RecordSuccess() {
	c.Requests++
	c.Successes++
	c.TotalSuccesses++
	c.ConsecutiveSuccesses++
	c.ConsecutiveFailures = 0
	c.LastSuccess = time.Now()
}

// RecordFailure records a failed request
func (c *Counts) RecordFailure() {
	c.Requests++
	c.Failures++
	c.TotalFailures++
	c.ConsecutiveFailures++
	c.ConsecutiveSuccesses = 0
	c.LastFailure = time.Now()
}

// RecordTimeout records a request timeout
func (c *Counts) RecordTimeout() {
	c.Requests++
	c.Failures++
	c.TotalFailures++
	c.Timeout++
	c.ConsecutiveFailures++
	c.ConsecutiveSuccesses = 0
	c.LastTimeout = time.Now()
	c.LastFailure = time.Now()
}

// RecordRejected records a rejected request
func (c *Counts) RecordRejected() {
	c.Rejected++
}

// RecordShortCircuited records a short-circuited request
func (c *Counts) RecordShortCircuited() {
	c.ShortCircuited++
}

// Reset resets all counters
func (c *Counts) Reset() {
	c.Requests = 0
	c.Successes = 0
	c.Failures = 0
	c.ConsecutiveSuccesses = 0
	c.ConsecutiveFailures = 0
	c.Timeout = 0
	c.ShortCircuited = 0
	c.Rejected = 0
}

// ResetTimestamps resets all timestamps
func (c *Counts) ResetTimestamps() {
	c.LastSuccess = time.Time{}
	c.LastFailure = time.Time{}
	c.LastTimeout = time.Time{}
}
