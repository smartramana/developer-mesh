#!/bin/bash

# Test if server is running and responding
HEALTH_OUTPUT=$(curl -s http://localhost:8080/health)
echo "Health endpoint output:"
echo $HEALTH_OUTPUT

# Check specific components that should be missing after removing webhooks
if [[ $HEALTH_OUTPUT == *"\"harness\""* ]]; then
  echo "ERROR: 'harness' component still present in health check"
  exit 1
fi

if [[ $HEALTH_OUTPUT == *"\"sonarqube\""* ]]; then
  echo "ERROR: 'sonarqube' component still present in health check"
  exit 1
fi

if [[ $HEALTH_OUTPUT == *"\"artifactory\""* ]]; then
  echo "ERROR: 'artifactory' component still present in health check"
  exit 1
fi

if [[ $HEALTH_OUTPUT == *"\"xray\""* ]]; then
  echo "ERROR: 'xray' component still present in health check"
  exit 1
fi

echo "Test passed!"
exit 0
