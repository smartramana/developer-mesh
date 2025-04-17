#!/bin/bash

# Configuration
WEBHOOK_URL="http://localhost:8080/webhook/github"
WEBHOOK_SECRET="test-webhook-secret"
PAYLOAD='{"repository":{"full_name":"test/repo"}}'
EVENT_TYPE="ping"

# Generate the SHA-256 HMAC signature
SIGNATURE=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | sed 's/^.* //')

echo "Payload: $PAYLOAD"
echo "Secret: $WEBHOOK_SECRET"
echo "Signature: $SIGNATURE"

# Send the webhook request
curl -i -X POST "$WEBHOOK_URL" \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: $EVENT_TYPE" \
  -H "X-Hub-Signature-256: sha256=$SIGNATURE" \
  -d "$PAYLOAD"
