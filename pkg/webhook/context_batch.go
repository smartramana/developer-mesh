package webhook

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// BatchProcessor handles batch operations for context lifecycle
type BatchProcessor struct {
	manager       *ContextLifecycleManager
	batchSize     int
	flushInterval time.Duration
	logger        observability.Logger

	// Batch buffers
	transitionBuffer []TransitionRequest
	bufferMu         sync.Mutex

	// Control
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// TransitionRequest represents a state transition request
type TransitionRequest struct {
	TenantID  string
	ContextID string
	FromState ContextState
	ToState   ContextState
	Priority  int // Higher priority processed first
}

// NewBatchProcessor creates a new batch processor
func NewBatchProcessor(manager *ContextLifecycleManager, batchSize int, flushInterval time.Duration) *BatchProcessor {
	bp := &BatchProcessor{
		manager:          manager,
		batchSize:        batchSize,
		flushInterval:    flushInterval,
		logger:           manager.logger,
		transitionBuffer: make([]TransitionRequest, 0, batchSize),
		stopCh:           make(chan struct{}),
	}

	// Start batch processor
	bp.wg.Add(1)
	go bp.processingLoop()

	return bp
}

// AddTransition adds a transition request to the batch
func (bp *BatchProcessor) AddTransition(req TransitionRequest) error {
	bp.bufferMu.Lock()
	defer bp.bufferMu.Unlock()

	// Check if buffer is full
	if len(bp.transitionBuffer) >= bp.batchSize {
		// Flush synchronously if buffer is full
		if err := bp.flushTransitions(); err != nil {
			return fmt.Errorf("failed to flush full buffer: %w", err)
		}
	}

	bp.transitionBuffer = append(bp.transitionBuffer, req)
	return nil
}

// processingLoop is the main batch processing loop
func (bp *BatchProcessor) processingLoop() {
	defer bp.wg.Done()

	ticker := time.NewTicker(bp.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			bp.bufferMu.Lock()
			if len(bp.transitionBuffer) > 0 {
				if err := bp.flushTransitions(); err != nil {
					bp.logger.Error("Failed to flush transitions", map[string]interface{}{
						"error": err.Error(),
					})
				}
			}
			bp.bufferMu.Unlock()

		case <-bp.stopCh:
			// Flush remaining transitions before stopping
			bp.bufferMu.Lock()
			if len(bp.transitionBuffer) > 0 {
				_ = bp.flushTransitions()
			}
			bp.bufferMu.Unlock()
			return
		}
	}
}

// flushTransitions processes all buffered transitions
func (bp *BatchProcessor) flushTransitions() error {
	if len(bp.transitionBuffer) == 0 {
		return nil
	}

	startTime := time.Now()

	// Copy buffer and clear it
	transitions := make([]TransitionRequest, len(bp.transitionBuffer))
	copy(transitions, bp.transitionBuffer)
	bp.transitionBuffer = bp.transitionBuffer[:0]

	// Group by transition type for efficient processing
	grouped := bp.groupTransitions(transitions)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	var wg sync.WaitGroup
	errChan := make(chan error, len(grouped))

	// Process each group in parallel
	for transitionType, requests := range grouped {
		wg.Add(1)
		go func(tType string, reqs []TransitionRequest) {
			defer wg.Done()

			batchStart := time.Now()
			if err := bp.processBatchByType(ctx, tType, reqs); err != nil {
				errChan <- fmt.Errorf("failed to process %s transitions: %w", tType, err)
			} else if bp.manager.metricsRecorder != nil {
				// Record batch processing metrics on success
				bp.manager.metricsRecorder.RecordBatchProcessing(
					tType,
					len(reqs),
					time.Since(batchStart),
					len(reqs), // Assuming all succeeded if no error
					0,
				)
			}
		}(transitionType, requests)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return &BatchProcessingError{
			BatchSize:    len(transitions),
			FailureCount: len(errs),
			Errors:       errs,
		}
	}

	bp.logger.Info("Successfully processed batch transitions", map[string]interface{}{
		"total_count": len(transitions),
		"groups":      len(grouped),
		"duration":    time.Since(startTime).String(),
	})

	return nil
}

// groupTransitions groups transitions by type
func (bp *BatchProcessor) groupTransitions(transitions []TransitionRequest) map[string][]TransitionRequest {
	grouped := make(map[string][]TransitionRequest)

	for _, t := range transitions {
		key := fmt.Sprintf("%s_to_%s", t.FromState, t.ToState)
		grouped[key] = append(grouped[key], t)
	}

	return grouped
}

// processBatchByType processes a batch of transitions of the same type
func (bp *BatchProcessor) processBatchByType(ctx context.Context, transitionType string, requests []TransitionRequest) error {
	switch transitionType {
	case "hot_to_warm":
		return bp.batchTransitionHotToWarm(ctx, requests)
	case "warm_to_cold":
		return bp.batchTransitionWarmToCold(ctx, requests)
	case "cold_to_warm":
		return bp.batchTransitionColdToWarm(ctx, requests)
	default:
		return fmt.Errorf("unknown transition type: %s", transitionType)
	}
}

// batchTransitionHotToWarm efficiently transitions multiple contexts from hot to warm
func (bp *BatchProcessor) batchTransitionHotToWarm(ctx context.Context, requests []TransitionRequest) error {
	startTime := time.Now()

	// Process in chunks to avoid overwhelming Redis
	const chunkSize = 20
	successCount := 0
	failureCount := 0

	for i := 0; i < len(requests); i += chunkSize {
		end := i + chunkSize
		if end > len(requests) {
			end = len(requests)
		}

		chunk := requests[i:end]
		success, failure := bp.processHotToWarmChunk(ctx, nil, chunk)
		successCount += success
		failureCount += failure
	}

	// Update metrics
	bp.manager.metrics.mu.Lock()
	bp.manager.metrics.HotContexts -= int64(successCount)
	bp.manager.metrics.WarmContexts += int64(successCount)
	bp.manager.metrics.TotalTransitions += int64(successCount)
	bp.manager.metrics.mu.Unlock()

	// Record batch processing metrics
	if bp.manager.metricsRecorder != nil {
		bp.manager.metricsRecorder.RecordBatchProcessing(
			"hot_to_warm",
			len(requests),
			time.Since(startTime),
			successCount,
			failureCount,
		)
	}

	bp.logger.Info("Batch transitioned contexts from hot to warm", map[string]interface{}{
		"requested": len(requests),
		"succeeded": successCount,
		"failed":    failureCount,
		"duration":  time.Since(startTime).String(),
	})

	if failureCount > 0 {
		return fmt.Errorf("%d transitions failed", failureCount)
	}

	return nil
}

// processHotToWarmChunk processes a chunk of hot to warm transitions
func (bp *BatchProcessor) processHotToWarmChunk(ctx context.Context, _ interface{}, chunk []TransitionRequest) (int, int) {
	// Get the Redis client
	client := bp.manager.redisClient.GetClient()
	// Use pipeline for efficiency
	pipe := client.Pipeline()
	successCount := 0
	failureCount := 0

	// Track acquired locks to release after pipeline execution
	acquiredLocks := make([]*ContextLock, 0, len(chunk))

	for _, req := range chunk {
		// Try to acquire lock (non-blocking)
		lock, err := bp.manager.TryAcquireContextLock(ctx, req.TenantID, req.ContextID)
		if err != nil {
			failureCount++
			continue
		}

		// Prepare keys
		hotKey := fmt.Sprintf("context:hot:%s:%s", req.TenantID, req.ContextID)
		warmKey := fmt.Sprintf("context:warm:%s:%s", req.TenantID, req.ContextID)

		// Get context data
		hotData, err := client.Get(ctx, hotKey).Bytes()
		if err != nil {
			_ = lock.Release(ctx)
			failureCount++
			continue
		}

		// Compress for warm storage
		compressed, _, err := bp.manager.compression.CompressWithSemantics(hotData)
		if err != nil {
			_ = lock.Release(ctx)
			failureCount++
			continue
		}

		// Add pipeline commands
		pipe.Set(ctx, warmKey, compressed, bp.manager.config.WarmDuration)
		pipe.Del(ctx, hotKey)
		pipe.ZRem(ctx, transitionSetHotToWarm, fmt.Sprintf("%s:%s", req.TenantID, req.ContextID))

		// Track lock for later release
		acquiredLocks = append(acquiredLocks, lock)
		successCount++
	}

	// Execute pipeline
	if successCount > 0 {
		if _, err := pipe.Exec(ctx); err != nil {
			bp.logger.Error("Pipeline execution failed", map[string]interface{}{
				"error": err.Error(),
			})
			// Release all locks on error
			for _, lock := range acquiredLocks {
				_ = lock.Release(ctx)
			}
			return 0, len(chunk)
		}
	}

	// Release all locks after successful pipeline execution
	for _, lock := range acquiredLocks {
		if err := lock.Release(ctx); err != nil {
			bp.logger.Warn("Failed to release lock", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return successCount, failureCount
}

// batchTransitionWarmToCold efficiently transitions multiple contexts from warm to cold
func (bp *BatchProcessor) batchTransitionWarmToCold(ctx context.Context, requests []TransitionRequest) error {
	successCount := 0
	failureCount := 0

	// Process each request (cold storage operations can't be batched as easily)
	for _, req := range requests {
		if err := bp.manager.TransitionToCold(ctx, req.TenantID, req.ContextID); err != nil {
			bp.logger.Error("Failed to transition to cold", map[string]interface{}{
				"tenant_id":  req.TenantID,
				"context_id": req.ContextID,
				"error":      err.Error(),
			})
			failureCount++
		} else {
			successCount++
		}
	}

	bp.logger.Info("Batch transitioned contexts from warm to cold", map[string]interface{}{
		"requested": len(requests),
		"succeeded": successCount,
		"failed":    failureCount,
	})

	if failureCount > 0 {
		return fmt.Errorf("%d transitions failed", failureCount)
	}

	return nil
}

// batchTransitionColdToWarm handles batch transitions from cold to warm storage
func (bp *BatchProcessor) batchTransitionColdToWarm(ctx context.Context, requests []TransitionRequest) error {
	// This is used when multiple contexts need to be warmed up
	// For example, when loading a related set of contexts
	successCount := 0
	failureCount := 0

	for _, req := range requests {
		// Retrieve from cold and promote to warm
		contextData, err := bp.manager.retrieveFromCold(ctx, req.TenantID, req.ContextID)
		if err != nil {
			bp.logger.Error("Failed to retrieve from cold", map[string]interface{}{
				"tenant_id":  req.TenantID,
				"context_id": req.ContextID,
				"error":      err.Error(),
			})
			failureCount++
			continue
		}

		// The retrieveFromCold method already promotes to warm
		_ = contextData
		successCount++
	}

	bp.logger.Info("Batch transitioned contexts from cold to warm", map[string]interface{}{
		"requested": len(requests),
		"succeeded": successCount,
		"failed":    failureCount,
	})

	if failureCount > 0 {
		return fmt.Errorf("%d transitions failed", failureCount)
	}

	return nil
}

// Stop gracefully stops the batch processor
func (bp *BatchProcessor) Stop() {
	close(bp.stopCh)
	bp.wg.Wait()
}
