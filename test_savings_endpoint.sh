#!/bin/bash

# Test script for the /savings endpoint
# This demonstrates the cache analytics and debugging features

echo "Testing Autocache /savings Endpoint"
echo "===================================="
echo ""

if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "Error: ANTHROPIC_API_KEY environment variable is not set"
    exit 1
fi

# Make a few test requests
echo "1. Making small request (no cache)..."
curl -s -X POST http://localhost:8080/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "max_tokens": 20,
    "messages": [{"role": "user", "content": "Hi"}]
  }' > /dev/null

echo "✓ Request 1 completed"
echo ""

echo "2. Making large request with tools (cache enabled)..."
curl -s -X POST http://localhost:8080/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d @test_data/scenarios/n8n_agent_workflow/input.json > /dev/null

echo "✓ Request 2 completed"
echo ""

echo "3. Making another request with system prompt..."
curl -s -X POST http://localhost:8080/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d @test_data/scenarios/system_and_tools/input.json > /dev/null

echo "✓ Request 3 completed"
echo ""

echo "===================================="
echo "Fetching /savings Analytics"
echo "===================================="
echo ""

SAVINGS=$(curl -s http://localhost:8080/savings)

echo "AGGREGATED STATISTICS:"
echo "---------------------"
echo "$SAVINGS" | jq '.aggregated_stats'
echo ""

echo "DEBUG INFO (Where cache was injected):"
echo "--------------------------------------"
echo "$SAVINGS" | jq '.debug_info'
echo ""

echo "CONFIGURATION:"
echo "-------------"
echo "$SAVINGS" | jq '.config'
echo ""

echo "RECENT REQUESTS (Sample):"
echo "------------------------"
echo "$SAVINGS" | jq '.recent_requests | length as $count | {
  total_requests: $count,
  latest_request: .[-1] | {
    timestamp,
    model,
    cache_injected,
    total_tokens,
    cached_tokens,
    cache_ratio,
    breakpoints: .breakpoints | map({position, type, tokens, ttl})
  }
}'
echo ""

echo "DETAILED BREAKPOINT ANALYSIS:"
echo "----------------------------"
echo "$SAVINGS" | jq '.recent_requests[] | select(.cache_injected == true) | {
  timestamp,
  model,
  breakpoints: .breakpoints | map("[\(.type)] \(.position): \(.tokens) tokens (\(.ttl) TTL)")
}'
echo ""

echo "===================================="
echo "✅ /savings endpoint test complete!"
echo ""
echo "The endpoint shows:"
echo "  - Aggregated stats (total requests, cache ratio, savings)"
echo "  - Debug info (breakpoints by type, average tokens)"
echo "  - Recent request history with full metadata"
echo "  - Where cache control was injected (system/tools/content)"
