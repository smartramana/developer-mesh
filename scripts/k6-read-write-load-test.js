import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const readLatency = new Trend('read_latency_ms');
const writeLatency = new Trend('write_latency_ms');
const deleteLatency = new Trend('delete_latency_ms');
const writeErrors = new Counter('write_errors');
const conflictErrors = new Counter('conflict_errors');

// Test configuration - Mixed read/write workload
export const options = {
  stages: [
    { duration: '2m', target: 20 },   // Ramp up to 20 users
    { duration: '5m', target: 50 },   // Ramp up to 50 users
    { duration: '10m', target: 100 }, // Ramp up to 100 users
    { duration: '5m', target: 100 },  // Stay at 100 users
    { duration: '2m', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    'http_req_duration{type:read}': ['p(99)<100'],   // Reads < 100ms
    'http_req_duration{type:write}': ['p(99)<200'],  // Writes < 200ms
    errors: ['rate<0.01'],                            // Error rate < 1%
    write_errors: ['count<50'],                       // Less than 50 write errors
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8081';
const API_KEY = __ENV.API_KEY || 'test-token';
const TENANT_ID = __ENV.TENANT_ID || `load-test-tenant-${Math.floor(Math.random() * 10)}`;

// Track resources for cleanup
let createdResources = {
  contexts: [],
  agents: [],
  models: []
};

// Helper for authenticated requests with timing
function timedRequest(method, endpoint, payload = null, tags = {}) {
  const params = {
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'X-Tenant-ID': TENANT_ID,
      'Content-Type': 'application/json',
    },
    tags: tags,
  };

  const start = Date.now();
  let response;
  
  try {
    if (payload) {
      response = http[method.toLowerCase()](`${BASE_URL}${endpoint}`, JSON.stringify(payload), params);
    } else {
      response = http[method.toLowerCase()](`${BASE_URL}${endpoint}`, params);
    }
    
    const duration = Date.now() - start;
    
    // Track latency by operation type
    if (tags.type === 'read') {
      readLatency.add(duration);
    } else if (tags.type === 'write') {
      writeLatency.add(duration);
    } else if (tags.type === 'delete') {
      deleteLatency.add(duration);
    }
    
    // Check for errors
    const success = check(response, {
      'status is 2xx': (r) => r.status >= 200 && r.status < 300,
    });
    
    if (!success) {
      errorRate.add(1);
      if (tags.type === 'write') {
        writeErrors.add(1);
      }
      if (response.status === 409) {
        conflictErrors.add(1);
      }
    } else {
      errorRate.add(0);
    }
    
    return response;
  } catch (e) {
    errorRate.add(1);
    if (tags.type === 'write') {
      writeErrors.add(1);
    }
    return null;
  }
}

// Workload distribution:
// 60% reads, 30% writes, 10% deletes (realistic production pattern)
export default function () {
  const workloadType = Math.random();
  
  // READ OPERATIONS (60%)
  if (workloadType < 0.6) {
    // List operations
    if (Math.random() < 0.5) {
      // List contexts with pagination
      const page = Math.floor(Math.random() * 5);
      timedRequest('GET', `/api/v1/contexts?limit=20&offset=${page * 20}`, null, { type: 'read' });
      sleep(0.5);
      
      // List agents
      timedRequest('GET', '/api/v1/agents?limit=10', null, { type: 'read' });
      sleep(0.5);
    } else {
      // Get specific resources
      if (createdResources.contexts.length > 0) {
        const randomContext = createdResources.contexts[Math.floor(Math.random() * createdResources.contexts.length)];
        timedRequest('GET', `/api/v1/contexts/${randomContext}`, null, { type: 'read' });
      }
      
      if (createdResources.agents.length > 0) {
        const randomAgent = createdResources.agents[Math.floor(Math.random() * createdResources.agents.length)];
        timedRequest('GET', `/api/v1/agents/${randomAgent}`, null, { type: 'read' });
      }
    }
    
    sleep(1 + Math.random() * 2);
  }
  
  // WRITE OPERATIONS (30%)
  else if (workloadType < 0.9) {
    const writeType = Math.random();
    
    // Create new resources (40% of writes)
    if (writeType < 0.4) {
      // Create context
      const contextPayload = {
        agent_id: `agent-${__VU}-${Date.now()}`,
        model_id: ['gpt-4', 'gpt-3.5-turbo', 'claude-2'][Math.floor(Math.random() * 3)],
        max_tokens: 4000,
        metadata: {
          source: 'load-test',
          vu: __VU,
          iteration: __ITER,
          timestamp: new Date().toISOString(),
        }
      };
      
      const createResponse = timedRequest('POST', '/api/v1/contexts', contextPayload, { type: 'write' });
      
      if (createResponse && createResponse.status === 201) {
        const context = JSON.parse(createResponse.body);
        createdResources.contexts.push(context.id);
        
        // Keep array size manageable
        if (createdResources.contexts.length > 100) {
          createdResources.contexts.shift();
        }
      }
    }
    // Update existing resources (40% of writes)
    else if (writeType < 0.8) {
      if (createdResources.contexts.length > 0) {
        const contextId = createdResources.contexts[Math.floor(Math.random() * createdResources.contexts.length)];
        
        const updatePayload = {
          content: [
            {
              role: 'user',
              content: `Load test message ${__ITER} from VU ${__VU}`,
            }
          ],
          metadata: {
            updated_at: new Date().toISOString(),
            update_count: __ITER,
          }
        };
        
        timedRequest('PUT', `/api/v1/contexts/${contextId}`, updatePayload, { type: 'write' });
      }
    }
    // Bulk operations (20% of writes)
    else {
      if (createdResources.contexts.length >= 3) {
        const bulkPayload = [];
        for (let i = 0; i < 3; i++) {
          const contextId = createdResources.contexts[Math.floor(Math.random() * createdResources.contexts.length)];
          bulkPayload.push({
            id: contextId,
            metadata: {
              bulk_update: true,
              timestamp: new Date().toISOString(),
            }
          });
        }
        
        timedRequest('PUT', '/api/v1/contexts/bulk', bulkPayload, { type: 'write' });
      }
    }
    
    sleep(2 + Math.random() * 3);
  }
  
  // DELETE OPERATIONS (10%)
  else {
    if (createdResources.contexts.length > 10) {
      // Delete oldest context
      const contextId = createdResources.contexts.shift();
      timedRequest('DELETE', `/api/v1/contexts/${contextId}`, null, { type: 'delete' });
    }
    
    sleep(1 + Math.random() * 2);
  }
  
  // Occasional agent/model creation (1% chance)
  if (Math.random() < 0.01) {
    // Create agent
    const agentPayload = {
      name: `load-test-agent-${Date.now()}`,
      description: `Created by VU ${__VU} at iteration ${__ITER}`,
    };
    
    const agentResponse = timedRequest('POST', '/api/v1/agents', agentPayload, { type: 'write' });
    if (agentResponse && agentResponse.status === 201) {
      const agent = JSON.parse(agentResponse.body);
      createdResources.agents.push(agent.id);
    }
  }
  
  // Health check (1% of requests)
  if (Math.random() < 0.01) {
    const healthRes = http.get(`${BASE_URL}/health`);
    check(healthRes, {
      'health check ok': (r) => r.status === 200,
    });
  }
}

// Setup
export function setup() {
  console.log('Starting mixed read-write load test...');
  console.log(`Target URL: ${BASE_URL}`);
  console.log(`Tenant ID: ${TENANT_ID}`);
  console.log('Workload: 60% reads, 30% writes, 10% deletes');
  
  // Verify system health
  const healthRes = http.get(`${BASE_URL}/health`);
  if (!healthRes || healthRes.status !== 200) {
    throw new Error('System is not healthy');
  }
  
  // Create initial test data
  console.log('Creating initial test data...');
  const params = {
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'X-Tenant-ID': TENANT_ID,
      'Content-Type': 'application/json',
    }
  };
  
  // Create a few initial contexts
  for (let i = 0; i < 5; i++) {
    const res = http.post(`${BASE_URL}/api/v1/contexts`, 
      JSON.stringify({
        agent_id: `seed-agent-${i}`,
        model_id: 'gpt-4',
        metadata: { seed: true }
      }), params);
      
    if (res.status === 201) {
      const context = JSON.parse(res.body);
      createdResources.contexts.push(context.id);
    }
  }
  
  return { startTime: Date.now(), initialContexts: createdResources.contexts.length };
}

// Teardown
export function teardown(data) {
  const duration = (Date.now() - data.startTime) / 1000 / 60;
  console.log(`\nLoad test completed after ${duration.toFixed(1)} minutes`);
  console.log(`Created contexts: ${createdResources.contexts.length}`);
  console.log(`Created agents: ${createdResources.agents.length}`);
  
  // Cleanup (optional - comment out to keep test data)
  console.log('Cleaning up test data...');
  const params = {
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
      'X-Tenant-ID': TENANT_ID,
    }
  };
  
  // Delete contexts
  createdResources.contexts.forEach(id => {
    http.del(`${BASE_URL}/api/v1/contexts/${id}`, null, params);
  });
  
  // Delete agents
  createdResources.agents.forEach(id => {
    http.del(`${BASE_URL}/api/v1/agents/${id}`, null, params);
  });
}