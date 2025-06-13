import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

// Custom metrics
const messagesSent = new Counter('websocket_messages_sent');
const messagesReceived = new Counter('websocket_messages_received');
const messageErrors = new Counter('websocket_message_errors');
const connectionErrors = new Counter('websocket_connection_errors');
const messageLatency = new Trend('websocket_message_latency');
const successRate = new Rate('websocket_success_rate');

// Test configuration
export const options = {
  stages: [
    { duration: '30s', target: 10 },   // Ramp up to 10 connections
    { duration: '1m', target: 50 },    // Ramp up to 50 connections
    { duration: '2m', target: 100 },   // Ramp up to 100 connections
    { duration: '1m', target: 100 },   // Stay at 100 connections
    { duration: '30s', target: 0 },    // Ramp down to 0
  ],
  thresholds: {
    'websocket_success_rate': ['rate>0.95'],
    'websocket_message_latency': ['p(95)<500', 'p(99)<1000'],
    'websocket_connection_errors': ['count<10'],
  },
};

// Test data
const BASE_URL = __ENV.MCP_SERVER_URL || 'http://localhost:8080';
const WS_URL = BASE_URL.replace('http://', 'ws://').replace('https://', 'wss://') + '/ws';
const API_KEY = __ENV.TEST_API_KEY || 'test-key-admin';

export default function () {
  const url = WS_URL;
  const params = {
    headers: {
      'Authorization': `Bearer ${API_KEY}`,
    },
    tags: { name: 'websocket_load_test' },
  };

  const res = ws.connect(url, params, function (socket) {
    socket.on('open', () => {
      console.log('WebSocket connection opened');

      // Send initialize message
      const initMsg = {
        id: uuidv4(),
        type: 0, // Request
        method: 'initialize',
        params: {
          name: `load-test-agent-${__VU}`,
          version: '1.0.0',
        },
      };
      
      socket.send(JSON.stringify(initMsg));
      messagesSent.add(1);
    });

    socket.on('message', (data) => {
      messagesReceived.add(1);
      
      try {
        const msg = JSON.parse(data);
        
        // Check for errors
        if (msg.error) {
          messageErrors.add(1);
          successRate.add(false);
          console.error(`Error response: ${msg.error.message}`);
          return;
        }
        
        successRate.add(true);
        
        // After initialization, send regular messages
        if (msg.id && msg.type === 1) { // Response
          // Send tool.list request
          const toolListMsg = {
            id: uuidv4(),
            type: 0, // Request
            method: 'tool.list',
          };
          
          const startTime = Date.now();
          socket.send(JSON.stringify(toolListMsg));
          messagesSent.add(1);
          
          // Track latency when we get the response
          socket.on('message', (responseData) => {
            try {
              const response = JSON.parse(responseData);
              if (response.id === toolListMsg.id) {
                const latency = Date.now() - startTime;
                messageLatency.add(latency);
              }
            } catch (e) {
              messageErrors.add(1);
            }
          });
          
          // Send context operations
          if (Math.random() < 0.3) { // 30% chance
            const contextMsg = {
              id: uuidv4(),
              type: 0, // Request
              method: 'context.create',
              params: {
                name: `test-context-${__VU}-${Date.now()}`,
                content: 'Load test context content',
              },
            };
            
            socket.send(JSON.stringify(contextMsg));
            messagesSent.add(1);
          }
        }
      } catch (e) {
        messageErrors.add(1);
        console.error(`Failed to parse message: ${e}`);
      }
    });

    socket.on('close', () => {
      console.log('WebSocket connection closed');
    });

    socket.on('error', (e) => {
      connectionErrors.add(1);
      console.error(`WebSocket error: ${e}`);
    });

    // Keep connection open and send periodic messages
    socket.setInterval(() => {
      const pingMsg = {
        id: uuidv4(),
        type: 0, // Request
        method: 'tool.list',
      };
      
      socket.send(JSON.stringify(pingMsg));
      messagesSent.add(1);
    }, 5000); // Every 5 seconds

    // Let the connection run for a while
    socket.setTimeout(() => {
      socket.close();
    }, 60000); // 60 seconds
  });

  check(res, {
    'WebSocket connection established': (r) => r && r.status === 101,
  });

  // Sleep to avoid overwhelming the server
  sleep(1);
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data, { indent: ' ', enableColors: true }),
    'websocket-load-test-results.json': JSON.stringify(data),
  };
}

// Helper function for text summary
function textSummary(data, options) {
  const { metrics } = data;
  let summary = '\n=== WebSocket Load Test Results ===\n\n';
  
  // Connection metrics
  summary += 'Connection Metrics:\n';
  summary += `  Total Connections: ${metrics.websocket_connection_errors ? metrics.websocket_connection_errors.values.count : 0}\n`;
  summary += `  Connection Errors: ${metrics.websocket_connection_errors ? metrics.websocket_connection_errors.values.count : 0}\n\n`;
  
  // Message metrics
  summary += 'Message Metrics:\n';
  summary += `  Messages Sent: ${metrics.websocket_messages_sent ? metrics.websocket_messages_sent.values.count : 0}\n`;
  summary += `  Messages Received: ${metrics.websocket_messages_received ? metrics.websocket_messages_received.values.count : 0}\n`;
  summary += `  Message Errors: ${metrics.websocket_message_errors ? metrics.websocket_message_errors.values.count : 0}\n`;
  summary += `  Success Rate: ${metrics.websocket_success_rate ? (metrics.websocket_success_rate.values.rate * 100).toFixed(2) : 0}%\n\n`;
  
  // Latency metrics
  if (metrics.websocket_message_latency) {
    summary += 'Latency Metrics (ms):\n';
    summary += `  Average: ${metrics.websocket_message_latency.values.avg.toFixed(2)}\n`;
    summary += `  Min: ${metrics.websocket_message_latency.values.min.toFixed(2)}\n`;
    summary += `  Max: ${metrics.websocket_message_latency.values.max.toFixed(2)}\n`;
    summary += `  P95: ${metrics.websocket_message_latency.values['p(95)'].toFixed(2)}\n`;
    summary += `  P99: ${metrics.websocket_message_latency.values['p(99)'].toFixed(2)}\n`;
  }
  
  return summary;
}