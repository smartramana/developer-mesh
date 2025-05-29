import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');

// Test configuration
export const options = {
  stages: [
    { duration: '2m', target: 10 },   // Ramp up to 10 users
    { duration: '5m', target: 50 },   // Ramp up to 50 users
    { duration: '10m', target: 100 }, // Ramp up to 100 users
    { duration: '5m', target: 100 },  // Stay at 100 users
    { duration: '2m', target: 0 },    // Ramp down to 0 users
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'], // 95% of requests under 500ms, 99% under 1s
    errors: ['rate<0.1'],                            // Error rate under 10%
    http_req_failed: ['rate<0.1'],                   // HTTP failure rate under 10%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';
const API_KEY = __ENV.API_KEY || 'test-token';
const TENANT_ID = __ENV.TENANT_ID || 'load-test-tenant';

// Helper function to make authenticated requests
function makeRequest(method, endpoint, payload = null) {
  const params = {
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'X-Tenant-ID': TENANT_ID,
      'Content-Type': 'application/json',
    },
  };

  let response;
  if (payload) {
    response = http[method.toLowerCase()](`${BASE_URL}${endpoint}`, JSON.stringify(payload), params);
  } else {
    response = http[method.toLowerCase()](`${BASE_URL}${endpoint}`, params);
  }

  // Track errors
  const success = check(response, {
    'status is 200-299': (r) => r.status >= 200 && r.status < 300,
  });
  
  errorRate.add(!success);
  
  return response;
}

export default function () {
  // Scenario 1: Health check (10% of traffic)
  if (Math.random() < 0.1) {
    const healthRes = http.get(`${BASE_URL}/health`);
    check(healthRes, {
      'health check status is 200': (r) => r.status === 200,
      'health check response time < 50ms': (r) => r.timings.duration < 50,
    });
  }

  // Scenario 2: List operations (40% of traffic)
  if (Math.random() < 0.4) {
    // List agents
    makeRequest('GET', '/api/v1/agents');
    sleep(0.5);

    // List models
    makeRequest('GET', '/api/v1/models');
    sleep(0.5);

    // List contexts with pagination
    makeRequest('GET', '/api/v1/contexts?limit=20&offset=0');
    sleep(1);
  }

  // Scenario 3: Create and retrieve context (30% of traffic)
  if (Math.random() < 0.3) {
    // Create context
    const createPayload = {
      agent_id: `agent-${__VU}-${Date.now()}`,
      model_id: 'gpt-4',
      metadata: {
        source: 'load-test',
        iteration: __ITER,
        vu: __VU,
      },
      max_tokens: 4000,
    };
    
    const createRes = makeRequest('POST', '/api/v1/contexts', createPayload);
    
    if (createRes.status === 201) {
      const context = JSON.parse(createRes.body);
      sleep(1);
      
      // Retrieve the created context
      makeRequest('GET', `/api/v1/contexts/${context.id}`);
      sleep(0.5);
      
      // Update context
      const updatePayload = {
        content: [
          {
            role: 'user',
            content: `Load test message ${__ITER}`,
          },
        ],
      };
      makeRequest('PUT', `/api/v1/contexts/${context.id}`, updatePayload);
    }
    
    sleep(2);
  }

  // Scenario 4: Vector search (15% of traffic)
  if (Math.random() < 0.15) {
    const searchPayload = {
      query: `test query ${Math.floor(Math.random() * 1000)}`,
      limit: 10,
      threshold: 0.7,
      content_type: 'document',
    };
    
    const searchRes = makeRequest('POST', '/api/v1/search/vector', searchPayload);
    check(searchRes, {
      'vector search response time < 1s': (r) => r.timings.duration < 1000,
    });
    
    sleep(2);
  }

  // Scenario 5: Concurrent operations (5% of traffic)
  if (Math.random() < 0.05) {
    const batch = http.batch([
      ['GET', `${BASE_URL}/api/v1/agents`, null, { headers: { 'Authorization': `Bearer ${API_KEY}`, 'X-Tenant-ID': TENANT_ID } }],
      ['GET', `${BASE_URL}/api/v1/models`, null, { headers: { 'Authorization': `Bearer ${API_KEY}`, 'X-Tenant-ID': TENANT_ID } }],
      ['GET', `${BASE_URL}/api/v1/tools`, null, { headers: { 'Authorization': `Bearer ${API_KEY}`, 'X-Tenant-ID': TENANT_ID } }],
    ]);
    
    check(batch[0], { 'batch request 1 success': (r) => r.status === 200 });
    check(batch[1], { 'batch request 2 success': (r) => r.status === 200 });
    check(batch[2], { 'batch request 3 success': (r) => r.status === 200 });
    
    sleep(3);
  }

  // Random sleep between iterations
  sleep(Math.random() * 2 + 1);
}

// Lifecycle hooks
export function setup() {
  // Verify the system is ready
  const healthRes = http.get(`${BASE_URL}/health`);
  if (healthRes.status !== 200) {
    throw new Error(`System is not healthy: ${healthRes.status}`);
  }
  
  console.log('Load test starting...');
  console.log(`Target URL: ${BASE_URL}`);
  console.log(`Total duration: 24 minutes`);
  console.log(`Peak load: 100 virtual users`);
}

export function teardown(data) {
  console.log('Load test completed');
}