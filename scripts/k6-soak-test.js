import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics for long-term monitoring
const errorRate = new Rate('errors');
const apiLatency = new Trend('api_latency');
const memoryTrend = new Trend('memory_usage_mb');

// Soak test configuration - 4 hours at constant load
export const options = {
  stages: [
    { duration: '5m', target: 50 },   // Ramp up to 50 users
    { duration: '4h', target: 50 },   // Stay at 50 users for 4 hours
    { duration: '5m', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'], // Performance should remain stable
    errors: ['rate<0.001'],                          // Very low error rate for soak test
    http_req_failed: ['rate<0.001'],                 // Very low failure rate
    api_latency: ['p(95)<500', 'p(99)<1000'],       // Custom metric thresholds
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';
const API_KEY = __ENV.API_KEY || 'test-token';
const TENANT_ID = __ENV.TENANT_ID || 'soak-test-tenant';

// Track test progress
let testStartTime = Date.now();
let contextCount = 0;

// Helper function for authenticated requests
function makeRequest(method, endpoint, payload = null) {
  const params = {
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'X-Tenant-ID': TENANT_ID,
      'Content-Type': 'application/json',
    },
    timeout: '30s', // Longer timeout for soak test
  };

  const start = Date.now();
  let response;
  
  try {
    if (payload) {
      response = http[method.toLowerCase()](`${BASE_URL}${endpoint}`, JSON.stringify(payload), params);
    } else {
      response = http[method.toLowerCase()](`${BASE_URL}${endpoint}`, params);
    }
    
    // Track latency
    apiLatency.add(Date.now() - start);
    
    // Track errors
    const success = check(response, {
      'status is 200-299': (r) => r.status >= 200 && r.status < 300,
      'no error in response': (r) => !r.body.includes('error'),
    });
    
    errorRate.add(!success);
    
    // Log errors for investigation
    if (!success) {
      console.error(`Request failed: ${method} ${endpoint} - Status: ${response.status}`);
    }
    
    return response;
  } catch (e) {
    console.error(`Request exception: ${method} ${endpoint} - ${e.message}`);
    errorRate.add(1);
    return null;
  }
}

// Monitor system health periodically
function checkSystemHealth() {
  const healthRes = http.get(`${BASE_URL}/health`);
  
  if (healthRes && healthRes.status === 200) {
    try {
      const health = JSON.parse(healthRes.body);
      
      // Log health status every 5 minutes
      const elapsed = Math.floor((Date.now() - testStartTime) / 1000 / 60);
      if (elapsed % 5 === 0) {
        console.log(`[${elapsed}min] System health: ${health.status}`);
        console.log(`[${elapsed}min] Contexts created: ${contextCount}`);
      }
      
      // Check component health
      if (health.components) {
        for (const [component, status] of Object.entries(health.components)) {
          if (status.status !== 'healthy') {
            console.warn(`Component ${component} is ${status.status}`);
          }
        }
      }
    } catch (e) {
      console.error('Failed to parse health response');
    }
  }
}

export default function () {
  // Realistic user behavior with think time
  const scenario = Math.random();
  
  // Scenario 1: Browse and read (60% of traffic)
  if (scenario < 0.6) {
    // List agents
    const agentsRes = makeRequest('GET', '/api/v1/agents?limit=20');
    sleep(2 + Math.random() * 3); // Think time
    
    // List models
    makeRequest('GET', '/api/v1/models?limit=10');
    sleep(2 + Math.random() * 3);
    
    // List contexts with pagination
    const page = Math.floor(Math.random() * 10);
    makeRequest('GET', `/api/v1/contexts?limit=20&offset=${page * 20}`);
    sleep(3 + Math.random() * 5);
    
    // Occasionally check specific context
    if (Math.random() < 0.3 && agentsRes && agentsRes.status === 200) {
      try {
        const contexts = JSON.parse(agentsRes.body);
        if (contexts.data && contexts.data.length > 0) {
          const randomContext = contexts.data[Math.floor(Math.random() * contexts.data.length)];
          makeRequest('GET', `/api/v1/contexts/${randomContext.id}`);
        }
      } catch (e) {
        // Ignore parsing errors
      }
    }
  }
  
  // Scenario 2: Create and manage contexts (30% of traffic)
  else if (scenario < 0.9) {
    // Create a new context
    const contextPayload = {
      agent_id: `soak-agent-${__VU}-${Date.now()}`,
      model_id: ['gpt-4', 'gpt-3.5-turbo', 'claude-2'][Math.floor(Math.random() * 3)],
      metadata: {
        source: 'soak-test',
        iteration: __ITER,
        vu: __VU,
        timestamp: new Date().toISOString(),
      },
      max_tokens: 4000,
    };
    
    const createRes = makeRequest('POST', '/api/v1/contexts', contextPayload);
    
    if (createRes && createRes.status === 201) {
      contextCount++;
      const context = JSON.parse(createRes.body);
      
      sleep(2 + Math.random() * 3);
      
      // Add messages to context
      for (let i = 0; i < Math.floor(Math.random() * 5) + 1; i++) {
        const messagePayload = {
          content: [
            {
              role: 'user',
              content: `Soak test message ${i}: ${Date.now()}`,
            },
          ],
        };
        
        makeRequest('PUT', `/api/v1/contexts/${context.id}`, messagePayload);
        sleep(1 + Math.random() * 2);
      }
      
      // Occasionally delete old contexts to prevent unbounded growth
      if (Math.random() < 0.1) {
        makeRequest('DELETE', `/api/v1/contexts/${context.id}`);
      }
    }
  }
  
  // Scenario 3: Search operations (10% of traffic)
  else {
    // Vector search
    const searchPayload = {
      query: `soak test query ${Math.floor(Math.random() * 100)}`,
      limit: 20,
      threshold: 0.7,
      content_type: 'document',
    };
    
    makeRequest('POST', '/api/v1/search/vector', searchPayload);
    sleep(3 + Math.random() * 5);
    
    // Text search
    const textSearchPayload = {
      query: `keyword ${Math.floor(Math.random() * 50)}`,
      filters: {
        model_id: 'gpt-4',
      },
    };
    
    makeRequest('POST', '/api/v1/search/text', textSearchPayload);
    sleep(2 + Math.random() * 3);
  }
  
  // Periodic health check (every 100 iterations)
  if (__ITER % 100 === 0) {
    checkSystemHealth();
  }
  
  // Base sleep between iterations
  sleep(5 + Math.random() * 10);
}

// Setup: verify system is ready
export function setup() {
  console.log('Starting 4-hour soak test...');
  console.log(`Target URL: ${BASE_URL}`);
  console.log('Configuration:');
  console.log('- Duration: 4 hours');
  console.log('- Virtual Users: 50');
  console.log('- Think time: 5-15 seconds');
  
  // Initial health check
  const healthRes = http.get(`${BASE_URL}/health`);
  if (!healthRes || healthRes.status !== 200) {
    throw new Error('System is not healthy at test start');
  }
  
  return { startTime: Date.now() };
}

// Teardown: final report
export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000 / 60;
  console.log(`\nSoak test completed after ${duration.toFixed(1)} minutes`);
  console.log(`Total contexts created: ${contextCount}`);
  
  // Final health check
  const healthRes = http.get(`${BASE_URL}/health`);
  if (healthRes && healthRes.status === 200) {
    console.log('System still healthy after soak test ✓');
  } else {
    console.log('System unhealthy after soak test ✗');
  }
}