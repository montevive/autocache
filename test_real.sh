#!/bin/bash

# Real API Testing Script for Autocache Proxy
# Tests actual Anthropic API integration with caching

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Configuration
PROXY_URL="http://localhost:8080"
TEST_RESULTS_DIR="test_results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
RESULTS_FILE="$TEST_RESULTS_DIR/real_test_$TIMESTAMP.log"

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $message" >> "$RESULTS_FILE"
}

# Function to make API request and capture headers
make_request() {
    local description="$1"
    local request_file="$2"
    local headers_file="$3"
    local response_file="$4"

    print_status $BLUE "📤 Making request: $description"

    # Make request and capture both headers and response
    local start_time=$(date +%s.%N)

    curl -s -w "\n%{http_code}\n%{time_total}\n" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ANTHROPIC_API_KEY" \
        -D "$headers_file" \
        -X POST \
        -d @"$request_file" \
        "$PROXY_URL/v1/messages" > "$response_file.tmp"

    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc -l)

    # Extract status code and time from response
    local status_code=$(tail -2 "$response_file.tmp" | head -1)
    local curl_time=$(tail -1 "$response_file.tmp")

    # Remove status and time from response
    head -n -2 "$response_file.tmp" > "$response_file"
    rm "$response_file.tmp"

    print_status $CYAN "   Status: $status_code | Duration: ${duration}s | Curl time: ${curl_time}s"

    if [ "$status_code" = "200" ]; then
        print_status $GREEN "✅ Request successful"
        return 0
    else
        print_status $RED "❌ Request failed with status $status_code"
        return 1
    fi
}

# Function to extract and display cache headers
analyze_cache_headers() {
    local headers_file="$1"
    local description="$2"

    print_status $YELLOW "📊 Cache Analysis: $description"

    # Extract cache headers
    local injected=$(grep -i "x-autocache-injected" "$headers_file" | cut -d' ' -f2- | tr -d '\r')
    local total_tokens=$(grep -i "x-autocache-total-tokens" "$headers_file" | cut -d' ' -f2- | tr -d '\r')
    local cached_tokens=$(grep -i "x-autocache-cached-tokens" "$headers_file" | cut -d' ' -f2- | tr -d '\r')
    local cache_ratio=$(grep -i "x-autocache-cache-ratio" "$headers_file" | cut -d' ' -f2- | tr -d '\r')
    local roi_percent=$(grep -i "x-autocache-roi-percent" "$headers_file" | cut -d' ' -f2- | tr -d '\r')
    local roi_break_even=$(grep -i "x-autocache-roi-breakeven" "$headers_file" | cut -d' ' -f2- | tr -d '\r')
    local roi_savings=$(grep -i "x-autocache-roi-savings" "$headers_file" | cut -d' ' -f2- | tr -d '\r')
    local strategy=$(grep -i "x-autocache-strategy" "$headers_file" | cut -d' ' -f2- | tr -d '\r')
    local breakpoints=$(grep -i "x-autocache-breakpoints" "$headers_file" | cut -d' ' -f2- | tr -d '\r')

    echo "   Cache Injected: $injected"
    echo "   Total Tokens: $total_tokens"
    echo "   Cached Tokens: $cached_tokens"
    echo "   Cache Ratio: $cache_ratio"
    echo "   ROI Percentage: $roi_percent%"
    echo "   Break-even Requests: $roi_break_even"
    echo "   Savings per Request: $roi_savings"
    echo "   Strategy: $strategy"
    echo "   Breakpoints: $breakpoints"

    # Save to results
    {
        echo "=== Cache Analysis: $description ==="
        echo "Cache Injected: $injected"
        echo "Total Tokens: $total_tokens"
        echo "Cached Tokens: $cached_tokens"
        echo "Cache Ratio: $cache_ratio"
        echo "ROI Percentage: $roi_percent%"
        echo "Break-even Requests: $roi_break_even"
        echo "Savings per Request: $roi_savings"
        echo "Strategy: $strategy"
        echo "Breakpoints: $breakpoints"
        echo ""
    } >> "$RESULTS_FILE"
}

# Function to extract usage from Anthropic response
analyze_usage() {
    local response_file="$1"
    local description="$2"

    print_status $YELLOW "💰 Usage Analysis: $description"

    # Extract usage information using jq if available, otherwise use grep
    if command -v jq &> /dev/null; then
        local input_tokens=$(jq -r '.usage.input_tokens // 0' "$response_file")
        local output_tokens=$(jq -r '.usage.output_tokens // 0' "$response_file")
        local cache_creation=$(jq -r '.usage.cache_creation_input_tokens // 0' "$response_file")
        local cache_read=$(jq -r '.usage.cache_read_input_tokens // 0' "$response_file")
    else
        # Fallback to grep/sed
        local input_tokens=$(grep -o '"input_tokens":[0-9]*' "$response_file" | cut -d':' -f2 || echo "0")
        local output_tokens=$(grep -o '"output_tokens":[0-9]*' "$response_file" | cut -d':' -f2 || echo "0")
        local cache_creation=$(grep -o '"cache_creation_input_tokens":[0-9]*' "$response_file" | cut -d':' -f2 || echo "0")
        local cache_read=$(grep -o '"cache_read_input_tokens":[0-9]*' "$response_file" | cut -d':' -f2 || echo "0")
    fi

    echo "   Input Tokens: $input_tokens"
    echo "   Output Tokens: $output_tokens"
    echo "   Cache Creation Tokens: $cache_creation"
    echo "   Cache Read Tokens: $cache_read"

    # Save to results
    {
        echo "=== Usage Analysis: $description ==="
        echo "Input Tokens: $input_tokens"
        echo "Output Tokens: $output_tokens"
        echo "Cache Creation Tokens: $cache_creation"
        echo "Cache Read Tokens: $cache_read"
        echo ""
    } >> "$RESULTS_FILE"
}

# Function to check if proxy is running
check_proxy() {
    print_status $BLUE "🔍 Checking if autocache proxy is running..."

    if curl -s "$PROXY_URL/health" > /dev/null; then
        print_status $GREEN "✅ Proxy is running at $PROXY_URL"
        return 0
    else
        print_status $RED "❌ Proxy is not responding at $PROXY_URL"
        print_status $YELLOW "💡 Start the proxy with: ./autocache"
        exit 1
    fi
}

# Function to check API key
check_api_key() {
    if [ -z "$ANTHROPIC_API_KEY" ]; then
        print_status $RED "❌ ANTHROPIC_API_KEY environment variable is not set"
        print_status $YELLOW "💡 Set your API key in .env file or export ANTHROPIC_API_KEY=sk-ant-..."
        exit 1
    fi

    print_status $GREEN "✅ API key configured"
}

# Main execution
main() {
    print_status $CYAN "🚀 Starting Real API Tests for Autocache Proxy"
    print_status $CYAN "=============================================="

    # Create results directory
    mkdir -p "$TEST_RESULTS_DIR"

    # Load environment variables
    if [ -f ".env" ]; then
        source .env
        print_status $GREEN "✅ Loaded .env configuration"
    else
        print_status $YELLOW "⚠️  No .env file found, using environment variables"
    fi

    # Pre-flight checks
    check_api_key
    check_proxy

    print_status $BLUE "📁 Test results will be saved to: $RESULTS_FILE"

    # Test 1: Large System Prompt Test
    print_status $CYAN "\n🧪 Test 1: Large System Prompt (Cacheable Content)"
    print_status $CYAN "=================================================="

    cat > "$TEST_RESULTS_DIR/request_large_system.json" << 'EOF'
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 150,
  "system": "You are an expert software engineering assistant with deep knowledge of programming languages, software architecture, design patterns, and best practices. Your role is to provide comprehensive, accurate, and practical guidance on all aspects of software development. When responding to questions, you should: 1) Provide clear, well-structured explanations that are appropriate for the user's level of expertise, 2) Include relevant code examples when applicable, using proper syntax highlighting and commenting, 3) Explain the reasoning behind your recommendations, including trade-offs and alternatives, 4) Reference established software engineering principles and industry standards, 5) Suggest best practices for code organization, testing, documentation, and maintainability, 6) When discussing architecture or design decisions, consider scalability, performance, security, and maintainability implications, 7) Be thorough in your analysis but concise in your communication, 8) Ask clarifying questions if the requirements are ambiguous or incomplete. You have expertise in multiple programming languages including Python, JavaScript, TypeScript, Java, C++, Go, Rust, and others. You understand various frameworks, libraries, databases, cloud platforms, and development tools. You stay current with modern development practices including DevOps, CI/CD, containerization, microservices, and cloud-native architectures.",
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "What are the key principles of clean code?"
        }
      ]
    }
  ]
}
EOF

    # First request (should create cache)
    make_request "Large system prompt - First request" \
        "$TEST_RESULTS_DIR/request_large_system.json" \
        "$TEST_RESULTS_DIR/headers_large_system_1.txt" \
        "$TEST_RESULTS_DIR/response_large_system_1.json"

    analyze_cache_headers "$TEST_RESULTS_DIR/headers_large_system_1.txt" "First request (cache creation)"
    analyze_usage "$TEST_RESULTS_DIR/response_large_system_1.json" "First request"

    # Second request (should use cache)
    print_status $BLUE "\n📤 Making identical request (should use cache)..."
    make_request "Large system prompt - Second request" \
        "$TEST_RESULTS_DIR/request_large_system.json" \
        "$TEST_RESULTS_DIR/headers_large_system_2.txt" \
        "$TEST_RESULTS_DIR/response_large_system_2.json"

    analyze_cache_headers "$TEST_RESULTS_DIR/headers_large_system_2.txt" "Second request (cache hit)"
    analyze_usage "$TEST_RESULTS_DIR/response_large_system_2.json" "Second request"

    # Test 2: Tools and System Test
    print_status $CYAN "\n🧪 Test 2: System + Tools (Multiple Cache Points)"
    print_status $CYAN "================================================"

    cat > "$TEST_RESULTS_DIR/request_system_tools.json" << 'EOF'
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 200,
  "system": "You are a helpful assistant with access to various tools. You can help users with calculations, weather information, web searches, and other tasks. When a user asks for something that requires a tool, use the appropriate tool and explain the results clearly. Always be helpful, accurate, and concise in your responses. If you're unsure about something, ask for clarification rather than making assumptions.",
  "tools": [
    {
      "name": "calculator",
      "description": "Perform mathematical calculations including basic arithmetic, algebraic operations, trigonometric functions, logarithms, and statistical calculations. Supports complex expressions with parentheses, variables, and mathematical constants like pi and e. Can handle both integer and floating-point numbers with high precision.",
      "input_schema": {
        "type": "object",
        "properties": {
          "expression": {
            "type": "string",
            "description": "The mathematical expression to evaluate (e.g., '2 + 2', 'sqrt(16)', 'sin(pi/2)')"
          }
        },
        "required": ["expression"]
      }
    },
    {
      "name": "get_weather",
      "description": "Get current weather conditions and forecasts for any location worldwide. Provides detailed information including temperature, humidity, wind speed, precipitation, visibility, and atmospheric pressure. Can also provide extended forecasts and weather alerts.",
      "input_schema": {
        "type": "object",
        "properties": {
          "location": {
            "type": "string",
            "description": "The city, state/province, and country (e.g., 'San Francisco, CA, USA')"
          },
          "units": {
            "type": "string",
            "enum": ["celsius", "fahrenheit", "kelvin"],
            "description": "Temperature units to use",
            "default": "celsius"
          }
        },
        "required": ["location"]
      }
    }
  ],
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "What's 15% of $1,250?"
        }
      ]
    }
  ]
}
EOF

    make_request "System + Tools test" \
        "$TEST_RESULTS_DIR/request_system_tools.json" \
        "$TEST_RESULTS_DIR/headers_system_tools.txt" \
        "$TEST_RESULTS_DIR/response_system_tools.json"

    analyze_cache_headers "$TEST_RESULTS_DIR/headers_system_tools.txt" "System + Tools"
    analyze_usage "$TEST_RESULTS_DIR/response_system_tools.json" "System + Tools"

    # Test 3: Strategy Comparison
    print_status $CYAN "\n🧪 Test 3: Strategy Comparison"
    print_status $CYAN "=============================="

    # Test different strategies by changing the strategy in real-time
    strategies=("conservative" "moderate" "aggressive")

    for strategy in "${strategies[@]}"; do
        print_status $BLUE "\n📊 Testing $strategy strategy..."

        # Create request with strategy-specific system prompt
        cat > "$TEST_RESULTS_DIR/request_strategy_$strategy.json" << EOF
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 100,
  "system": "You are an AI assistant configured for $strategy caching strategy testing. This system prompt is designed to be large enough to trigger caching mechanisms in the autocache proxy. The $strategy strategy should handle this content according to its specific rules and token thresholds. This prompt contains sufficient content to exceed the minimum token requirements for cache consideration, allowing us to test the effectiveness of different caching approaches and their impact on cost optimization and performance improvements.",
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "text",
          "text": "Explain the $strategy caching strategy briefly."
        }
      ]
    }
  ]
}
EOF

        # Set strategy via environment variable (note: this won't change running server)
        # In real testing, you'd restart the server with different strategies
        make_request "$strategy strategy test" \
            "$TEST_RESULTS_DIR/request_strategy_$strategy.json" \
            "$TEST_RESULTS_DIR/headers_strategy_$strategy.txt" \
            "$TEST_RESULTS_DIR/response_strategy_$strategy.json"

        analyze_cache_headers "$TEST_RESULTS_DIR/headers_strategy_$strategy.txt" "$strategy strategy"
        analyze_usage "$TEST_RESULTS_DIR/response_strategy_$strategy.json" "$strategy strategy"
    done

    # Test 4: No Caching (Bypass) Test
    print_status $CYAN "\n🧪 Test 4: Cache Bypass Test"
    print_status $CYAN "==========================="

    # Test with bypass header
    print_status $BLUE "📤 Making request with cache bypass header..."

    curl -s -w "\n%{http_code}\n" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $ANTHROPIC_API_KEY" \
        -H "X-Autocache-Bypass: true" \
        -D "$TEST_RESULTS_DIR/headers_bypass.txt" \
        -X POST \
        -d @"$TEST_RESULTS_DIR/request_large_system.json" \
        "$PROXY_URL/v1/messages" > "$TEST_RESULTS_DIR/response_bypass.json.tmp"

    head -n -1 "$TEST_RESULTS_DIR/response_bypass.json.tmp" > "$TEST_RESULTS_DIR/response_bypass.json"
    rm "$TEST_RESULTS_DIR/response_bypass.json.tmp"

    analyze_cache_headers "$TEST_RESULTS_DIR/headers_bypass.txt" "Bypass test (should not cache)"
    analyze_usage "$TEST_RESULTS_DIR/response_bypass.json" "Bypass test"

    # Final Summary
    print_status $CYAN "\n📋 Test Summary"
    print_status $CYAN "==============="

    print_status $GREEN "✅ All real API tests completed successfully!"
    print_status $BLUE "📊 Detailed results saved to: $RESULTS_FILE"
    print_status $BLUE "📁 All test files available in: $TEST_RESULTS_DIR/"

    # List generated files
    print_status $YELLOW "\n📄 Generated Files:"
    ls -la "$TEST_RESULTS_DIR/" | grep "$TIMESTAMP" || ls -la "$TEST_RESULTS_DIR/"

    print_status $CYAN "\n💡 Next Steps:"
    echo "   1. Review the results in $RESULTS_FILE"
    echo "   2. Compare cache headers between first and second requests"
    echo "   3. Verify actual cost savings in Anthropic Console"
    echo "   4. Test with different cache strategies by restarting proxy with different CACHE_STRATEGY"

    print_status $GREEN "\n🎉 Real API testing complete!"
}

# Run main function
main "$@"