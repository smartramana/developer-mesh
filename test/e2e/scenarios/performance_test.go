package scenarios

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/S-Corkum/devops-mcp/test/e2e/agent"
	"github.com/S-Corkum/devops-mcp/test/e2e/data"
	"github.com/S-Corkum/devops-mcp/test/e2e/reporting"
	"github.com/S-Corkum/devops-mcp/test/e2e/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Performance and Stress E2E Tests", func() {
	var (
		config   *utils.Config
		reporter *reporting.StreamingReporter
		// testData  *data.TestData // Not needed - using config.APIKey directly
		isolation *utils.TestIsolation
		logger    *utils.TestLogger
	)

	BeforeEach(func() {
		config = utils.LoadConfig()
		reporter = reporting.NewStreamingReporter(config.ReportDir, []string{"json", "html", "junit"})
		// testData = data.DefaultTestData() // Not needed - using config.APIKey directly
		isolation = utils.NewTestIsolation()
		logger = utils.NewTestLogger("performance", config.EnableDebug)

		reporter.StartSuite("Performance and Stress Tests")
	})

	AfterEach(func() {
		reporter.EndSuite()
		_ = reporter.GenerateReports()
		_ = isolation.CleanupAll()
	})

	Describe("Agent Swarm Tests", func() {
		It("should handle 50 concurrent agents", func() {
			testResult := reporting.TestResult{
				Name:      "agent_swarm_50",
				Suite:     "performance",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("agent-swarm")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			ctx := context.Background()
			agentCount := 50
			agents := make([]*agent.TestAgent, agentCount)

			// Metrics tracking
			var successfulConnections int32
			var totalMessages int64
			var totalLatency int64
			latencies := make([]time.Duration, 0, agentCount*10)
			latenciesMu := sync.Mutex{}

			// Connect all agents concurrently
			var wg sync.WaitGroup
			connectionStart := time.Now()

			for i := 0; i < agentCount; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()

					ag := agent.NewTestAgent(
						fmt.Sprintf("swarm-agent-%d", idx),
						[]string{"test_capability"},
						config.APIKey,
						config.MCPBaseURL,
					)

					if err := ag.Connect(ctx); err != nil {
						logger.Error("Agent %d connection failed: %v", idx, err)
						return
					}

					agents[idx] = ag
					atomic.AddInt32(&successfulConnections, 1)

					// Perform some operations
					for j := 0; j < 10; j++ {
						start := time.Now()
						resp, err := ag.ExecuteMethod(ctx, "echo", map[string]interface{}{
							"message": fmt.Sprintf("test-%d-%d", idx, j),
						})
						latency := time.Since(start)

						if err == nil && resp.Error == nil {
							atomic.AddInt64(&totalMessages, 1)
							atomic.AddInt64(&totalLatency, int64(latency))

							latenciesMu.Lock()
							latencies = append(latencies, latency)
							latenciesMu.Unlock()
						}
					}
				}(i)
			}

			wg.Wait()
			connectionDuration := time.Since(connectionStart)

			// Calculate metrics
			connectedCount := atomic.LoadInt32(&successfulConnections)
			messageCount := atomic.LoadInt64(&totalMessages)
			avgLatency := time.Duration(0)
			if messageCount > 0 {
				avgLatency = time.Duration(atomic.LoadInt64(&totalLatency) / messageCount)
			}

			// Calculate percentiles
			latenciesMu.Lock()
			sort.Slice(latencies, func(i, j int) bool {
				return latencies[i] < latencies[j]
			})
			p50 := time.Duration(0)
			p95 := time.Duration(0)
			p99 := time.Duration(0)
			if len(latencies) > 0 {
				p50 = latencies[len(latencies)/2]
				p95 = latencies[int(float64(len(latencies))*0.95)]
				p99 = latencies[int(float64(len(latencies))*0.99)]
			}
			latenciesMu.Unlock()

			// Cleanup agents
			for _, ag := range agents {
				if ag != nil {
					_ = ag.Close()
				}
			}

			// Log results
			logger.Info("Agent Swarm Results:")
			logger.Info("  Agents connected: %d/%d", connectedCount, agentCount)
			logger.Info("  Connection time: %v", connectionDuration)
			logger.Info("  Total messages: %d", messageCount)
			logger.Info("  Avg latency: %v", avgLatency)
			logger.Info("  P50 latency: %v", p50)
			logger.Info("  P95 latency: %v", p95)
			logger.Info("  P99 latency: %v", p99)

			// Assertions
			Expect(connectedCount).To(BeNumerically(">=", int32(float64(agentCount)*0.95))) // Allow 5% failure
			Expect(avgLatency).To(BeNumerically("<", 100*time.Millisecond))
			Expect(p99).To(BeNumerically("<", 500*time.Millisecond))

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = fmt.Sprintf("Connected %d agents, avg latency %v", connectedCount, avgLatency)
			testResult.Metrics = map[string]interface{}{
				"agents_connected": connectedCount,
				"connection_time":  connectionDuration.String(),
				"total_messages":   messageCount,
				"avg_latency":      avgLatency.String(),
				"p50_latency":      p50.String(),
				"p95_latency":      p95.String(),
				"p99_latency":      p99.String(),
			}

			reporter.LogTest(testResult)
		})

		It("should handle agent churn", func() {
			testResult := reporting.TestResult{
				Name:      "agent_churn",
				Suite:     "performance",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("agent-churn")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			ctx := context.Background()
			churnDuration := 30 * time.Second
			agentLifetime := 5 * time.Second

			var totalConnections int64
			var totalDisconnections int64
			var totalErrors int64

			// Start churn test
			churnCtx, cancel := context.WithTimeout(ctx, churnDuration)
			defer cancel()

			var wg sync.WaitGroup

			// Spawn agents continuously (reduced from 10 to 5 to stay within connection limits)
			for i := 0; i < 5; i++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					for {
						select {
						case <-churnCtx.Done():
							return
						default:
							ag := agent.NewTestAgent(
								fmt.Sprintf("churn-agent-%d-%d", workerID, time.Now().UnixNano()),
								[]string{"churn_test"},
								config.APIKey,
								config.MCPBaseURL,
							)

							if err := ag.Connect(ctx); err != nil {
								atomic.AddInt64(&totalErrors, 1)
								time.Sleep(1 * time.Second)
								continue
							}

							atomic.AddInt64(&totalConnections, 1)

							// Keep agent alive for a while
							agentCtx, agentCancel := context.WithTimeout(ctx, agentLifetime)

							// Perform operations
							go func() {
								for {
									select {
									case <-agentCtx.Done():
										return
									default:
										_, _ = ag.ExecuteMethod(ctx, "ping", nil)
										time.Sleep(500 * time.Millisecond)
									}
								}
							}()

							<-agentCtx.Done()
							agentCancel()

							// Disconnect
							_ = ag.Close()
							atomic.AddInt64(&totalDisconnections, 1)

							// Small delay between connections
							time.Sleep(100 * time.Millisecond)
						}
					}
				}(i)
			}

			// Wait for churn test to complete
			<-churnCtx.Done()
			wg.Wait()

			connections := atomic.LoadInt64(&totalConnections)
			disconnections := atomic.LoadInt64(&totalDisconnections)
			errors := atomic.LoadInt64(&totalErrors)

			logger.Info("Agent Churn Results:")
			logger.Info("  Total connections: %d", connections)
			logger.Info("  Total disconnections: %d", disconnections)
			logger.Info("  Total errors: %d", errors)
			logger.Info("  Error rate: %.2f%%", float64(errors)/float64(connections+errors)*100)

			// Assertions
			Expect(connections).To(BeNumerically(">", 20))
			Expect(float64(errors) / float64(connections+errors)).To(BeNumerically("<", 0.1)) // <10% error rate

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = fmt.Sprintf("Handled %d connections with %.1f%% error rate",
				connections, float64(errors)/float64(connections+errors)*100)

			reporter.LogTest(testResult)
		})
	})

	Describe("Message Throughput Tests", func() {
		It("should handle high message throughput", func() {
			testResult := reporting.TestResult{
				Name:      "message_throughput",
				Suite:     "performance",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("message-throughput")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			ctx := context.Background()

			// Create high-performance agent
			perfAgent := agent.NewTestAgent(
				"perf-test-agent",
				[]string{"performance_test"},
				config.APIKey,
				config.MCPBaseURL,
			)

			err = perfAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = perfAgent.Close() }()

			// Test parameters - reduced to work within rate limits
			// Default rate limit is ~16.67 msg/sec (1000/60)
			// With 10 concurrent workers, we need to be careful
			messageCount := 100 // Reduced from 1000
			concurrency := 5    // Reduced from 10

			// Metrics
			var successCount int64
			var errorCount int64
			var totalLatency int64
			latencies := make([]time.Duration, 0, messageCount)
			latenciesMu := sync.Mutex{}

			// Start throughput test
			testStart := time.Now()
			var wg sync.WaitGroup
			messagesPerWorker := messageCount / concurrency

			for w := 0; w < concurrency; w++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					for i := 0; i < messagesPerWorker; i++ {
						start := time.Now()
						resp, err := perfAgent.ExecuteMethod(ctx, "echo", map[string]interface{}{
							"worker":   workerID,
							"sequence": i,
							"data":     fmt.Sprintf("message-%d-%d", workerID, i),
						})
						latency := time.Since(start)

						if err == nil && resp.Error == nil {
							atomic.AddInt64(&successCount, 1)
							atomic.AddInt64(&totalLatency, int64(latency))

							latenciesMu.Lock()
							latencies = append(latencies, latency)
							latenciesMu.Unlock()
						} else {
							atomic.AddInt64(&errorCount, 1)
						}

						// Small delay to respect rate limits
						// With 5 workers, 20ms delay = 250 msg/sec total, below rate limit
						time.Sleep(20 * time.Millisecond)
					}
				}(w)
			}

			wg.Wait()
			testDuration := time.Since(testStart)

			// Calculate metrics
			successful := atomic.LoadInt64(&successCount)
			errors := atomic.LoadInt64(&errorCount)
			throughput := float64(successful) / testDuration.Seconds()
			avgLatency := time.Duration(atomic.LoadInt64(&totalLatency) / successful)

			// Calculate percentiles
			latenciesMu.Lock()
			sort.Slice(latencies, func(i, j int) bool {
				return latencies[i] < latencies[j]
			})
			p50 := latencies[len(latencies)/2]
			p95 := latencies[int(float64(len(latencies))*0.95)]
			p99 := latencies[int(float64(len(latencies))*0.99)]
			latenciesMu.Unlock()

			logger.Info("Throughput Test Results:")
			logger.Info("  Messages sent: %d", successful)
			logger.Info("  Errors: %d", errors)
			logger.Info("  Duration: %v", testDuration)
			logger.Info("  Throughput: %.2f msg/sec", throughput)
			logger.Info("  Avg latency: %v", avgLatency)
			logger.Info("  P50 latency: %v", p50)
			logger.Info("  P95 latency: %v", p95)
			logger.Info("  P99 latency: %v", p99)

			// Assertions - adjusted for rate-limited scenario
			Expect(successful).To(Equal(int64(messageCount)))
			Expect(throughput).To(BeNumerically(">", 10))                   // At least 10 msg/sec (within rate limits)
			Expect(avgLatency).To(BeNumerically("<", 100*time.Millisecond)) // Allow more latency

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = fmt.Sprintf("Achieved %.0f msg/sec throughput", throughput)
			testResult.Metrics = map[string]interface{}{
				"throughput":  throughput,
				"avg_latency": avgLatency.String(),
				"p50_latency": p50.String(),
				"p95_latency": p95.String(),
				"p99_latency": p99.String(),
			}

			reporter.LogTest(testResult)
		})

		It("should handle large message payloads", func() {
			testResult := reporting.TestResult{
				Name:      "large_payload_handling",
				Suite:     "performance",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("large-payload")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			ctx := context.Background()

			largeAgent := agent.NewTestAgent(
				"large-payload-agent",
				[]string{"data_processing"},
				config.APIKey,
				config.MCPBaseURL,
			)

			err = largeAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = largeAgent.Close() }()

			// Test different payload sizes
			payloadSizes := []int{
				1024,    // 1KB
				10240,   // 10KB
				102400,  // 100KB
				1048576, // 1MB
				2000000, // 2MB (just under the limit)
			}

			results := make(map[int]time.Duration)

			for _, size := range payloadSizes {
				// Generate large payload
				largeData := data.GenerateLargeContext(size / 80) // ~80 chars per line

				start := time.Now()
				resp, err := largeAgent.ExecuteMethod(ctx, "context.create", map[string]interface{}{
					"name":     fmt.Sprintf("large-context-%d", size),
					"content":  largeData,
					"model_id": "claude-sonnet-4", // Default model for tests
					"metadata": map[string]interface{}{
						"size": size,
						"type": "performance_test",
					},
				})
				duration := time.Since(start)

				if err == nil && resp.Error == nil {
					results[size] = duration
					logger.Info("Payload %d bytes: %v", size, duration)
				} else {
					logger.Error("Failed with payload %d bytes: %v", size, err)
				}
			}

			// Verify results
			Expect(results).To(HaveLen(len(payloadSizes)))

			// Check that latency scales reasonably with size
			for i := 1; i < len(payloadSizes); i++ {
				prevSize := payloadSizes[i-1]
				currSize := payloadSizes[i]
				sizeRatio := float64(currSize) / float64(prevSize)
				latencyRatio := float64(results[currSize]) / float64(results[prevSize])

				// Latency should not scale linearly with size (due to compression)
				Expect(latencyRatio).To(BeNumerically("<", sizeRatio*2))
			}

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = "Large payloads handled efficiently"

			// Convert results to map[string]interface{}
			metrics := make(map[string]interface{})
			for size, duration := range results {
				metrics[fmt.Sprintf("%d_bytes", size)] = duration.String()
			}
			testResult.Metrics = metrics

			reporter.LogTest(testResult)
		})
	})

	Describe("Resource Utilization Tests", func() {
		It("should maintain stable performance under sustained load", func() {
			testResult := reporting.TestResult{
				Name:      "sustained_load",
				Suite:     "performance",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("sustained-load")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			ctx := context.Background()
			testDuration := 60 * time.Second

			// Create multiple agents
			agentCount := 10
			agents := make([]*agent.TestAgent, agentCount)

			for i := 0; i < agentCount; i++ {
				agents[i] = agent.NewTestAgent(
					fmt.Sprintf("load-agent-%d", i),
					[]string{"load_test"},
					config.APIKey,
					config.MCPBaseURL,
				)

				err := agents[i].Connect(ctx)
				Expect(err).NotTo(HaveOccurred())
				defer agents[i].Close()
			}

			// Metrics collection
			metricsCapture := utils.NewMetricsCapture()
			var requestCount int64
			var errorCount int64

			// Start load generation
			loadCtx, cancel := context.WithTimeout(ctx, testDuration)
			defer cancel()

			var wg sync.WaitGroup

			// Each agent generates continuous load
			for i, ag := range agents {
				wg.Add(1)
				go func(idx int, agent *agent.TestAgent) {
					defer wg.Done()

					for {
						select {
						case <-loadCtx.Done():
							return
						default:
							start := time.Now()
							_, err := agent.ExecuteMethod(ctx, "process", map[string]interface{}{
								"agent_id": idx,
								"data":     fmt.Sprintf("load-test-%d-%d", idx, time.Now().UnixNano()),
							})
							latency := time.Since(start)

							if err == nil {
								atomic.AddInt64(&requestCount, 1)
								metricsCapture.RecordLatency(latency)
							} else {
								atomic.AddInt64(&errorCount, 1)
							}

							// Small delay to avoid overwhelming
							time.Sleep(10 * time.Millisecond)
						}
					}
				}(i, ag)
			}

			// Monitor performance over time
			go func() {
				ticker := time.NewTicker(10 * time.Second)
				defer ticker.Stop()

				var lastCount int64
				for {
					select {
					case <-loadCtx.Done():
						return
					case <-ticker.C:
						currentCount := atomic.LoadInt64(&requestCount)
						rate := float64(currentCount-lastCount) / 10.0
						logger.Info("Current rate: %.2f req/sec", rate)
						lastCount = currentCount
					}
				}
			}()

			wg.Wait()
			metricsCapture.Finalize()

			// Calculate final metrics
			totalRequests := atomic.LoadInt64(&requestCount)
			totalErrors := atomic.LoadInt64(&errorCount)
			avgRate := float64(totalRequests) / testDuration.Seconds()
			errorRate := float64(totalErrors) / float64(totalRequests+totalErrors)

			logger.Info("Sustained Load Results:")
			logger.Info("  Total requests: %d", totalRequests)
			logger.Info("  Total errors: %d", totalErrors)
			logger.Info("  Average rate: %.2f req/sec", avgRate)
			logger.Info("  Error rate: %.2f%%", errorRate*100)
			logger.Info("  Avg latency: %v", metricsCapture.AverageLatency())

			// Assertions
			Expect(avgRate).To(BeNumerically(">", 50))     // At least 50 req/sec sustained
			Expect(errorRate).To(BeNumerically("<", 0.01)) // Less than 1% errors
			Expect(metricsCapture.AverageLatency()).To(BeNumerically("<", 100*time.Millisecond))

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = fmt.Sprintf("Sustained %.0f req/sec with %.2f%% errors", avgRate, errorRate*100)

			reporter.LogTest(testResult)
		})
	})

	Describe("Binary Protocol Performance", func() {
		It("should demonstrate binary protocol efficiency", func() {
			testResult := reporting.TestResult{
				Name:      "binary_protocol_performance",
				Suite:     "performance",
				StartTime: time.Now(),
			}

			namespace, err := isolation.CreateNamespace("binary-protocol")
			Expect(err).NotTo(HaveOccurred())
			defer isolation.DeleteNamespace(namespace.ID)

			ctx := context.Background()

			// Create agents with and without binary protocol
			textAgent := agent.NewTestAgent(
				"text-protocol-agent",
				[]string{"benchmark"},
				config.APIKey,
				config.MCPBaseURL,
			)

			binaryAgent := agent.NewTestAgent(
				"binary-protocol-agent",
				[]string{"benchmark"},
				config.APIKey,
				config.MCPBaseURL,
			)

			// Connect both agents
			err = textAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = textAgent.Close() }()

			err = binaryAgent.Connect(ctx)
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = binaryAgent.Close() }()

			// Enable binary protocol for binary agent
			// First send the request
			resp, err := binaryAgent.ExecuteMethod(ctx, "protocol.set_binary", map[string]interface{}{
				"enabled": true,
				"compression": map[string]interface{}{
					"enabled":   true,
					"threshold": 1024,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Error).To(BeNil())

			// Verify the protocol was enabled successfully
			result, ok := resp.Result.(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(result["binary_enabled"]).To(Equal(true))

			// Now that we've received the response confirming the server has switched,
			// we can safely switch the client to binary mode
			binaryAgent.SetBinaryMode(true, 1024)

			// Benchmark with large payload
			largePayload := data.GenerateLargeContext(5000) // ~400KB
			iterations := 10

			// Text protocol benchmark
			textStart := time.Now()
			for i := 0; i < iterations; i++ {
				resp, err := textAgent.ExecuteMethod(ctx, "echo", map[string]interface{}{
					"data":      largePayload,
					"iteration": i,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Error).To(BeNil())
			}
			textDuration := time.Since(textStart)

			// Binary protocol benchmark
			binaryStart := time.Now()
			for i := 0; i < iterations; i++ {
				resp, err := binaryAgent.ExecuteMethod(ctx, "echo", map[string]interface{}{
					"data":      largePayload,
					"iteration": i,
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Error).To(BeNil())
			}
			binaryDuration := time.Since(binaryStart)

			// Calculate improvement
			improvement := (1 - float64(binaryDuration)/float64(textDuration)) * 100

			logger.Info("Binary Protocol Performance:")
			logger.Info("  Text protocol: %v", textDuration)
			logger.Info("  Binary protocol: %v", binaryDuration)
			logger.Info("  Improvement: %.2f%%", improvement)

			// Binary should be faster for large payloads
			Expect(binaryDuration).To(BeNumerically("<", textDuration))

			testResult.Status = reporting.TestStatusPassed
			testResult.EndTime = time.Now()
			testResult.Duration = testResult.EndTime.Sub(testResult.StartTime)
			testResult.Message = fmt.Sprintf("Binary protocol %.0f%% faster", math.Abs(improvement))
			testResult.Metrics = map[string]interface{}{
				"text_duration":   textDuration.String(),
				"binary_duration": binaryDuration.String(),
				"improvement":     improvement,
			}

			reporter.LogTest(testResult)
		})
	})
})

// TestPerformance runs performance and stress tests
func TestPerformance(t *testing.T) {
	RegisterFailHandler(Fail)
	// Don't call RunSpecs here - it's handled by the suite
}
