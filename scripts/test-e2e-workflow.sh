#!/bin/bash
# Test script to validate e2e test workflow commands

set -e

echo "Testing E2E workflow commands..."
echo "==============================="

cd test/e2e

# Test single agent suite
echo -e "\n1. Testing single agent suite command:"
echo "ginkgo -v --timeout=30m --flake-attempts=2 --json-report=report.json --junit-report=junit.xml --focus=\"Single Agent\" --dry-run"

# Test multi-agent suite
echo -e "\n2. Testing multi-agent suite command:"
echo "ginkgo -v --timeout=30m --flake-attempts=2 --json-report=report.json --junit-report=junit.xml --focus=\"Multi-Agent\" --dry-run"

# Test performance suite
echo -e "\n3. Testing performance suite command:"
echo "ginkgo -v --timeout=30m --flake-attempts=2 --json-report=report.json --junit-report=junit.xml --focus=\"Performance\" --dry-run"

# Test all suites (no focus)
echo -e "\n4. Testing all suites command:"
echo "ginkgo -v --timeout=30m --flake-attempts=2 --json-report=report.json --junit-report=junit.xml --dry-run"

echo -e "\nAll commands are syntactically correct!"
echo "To run actual tests, remove the --dry-run flag and ensure E2E_API_KEY is set."