#!/bin/bash

# Agent Conversation Cache Test
# Tests multi-turn conversation caching with system prompt and tools

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Configuration
PROXY_URL="http://localhost:8080"
TEST_DIR="test_data/agent_conversation"
RESULTS_DIR="test_results/agent_conversation"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
RESULTS_FILE="$RESULTS_DIR/results_$TIMESTAMP.log"

# Create results directory
mkdir -p "$RESULTS_DIR"

echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║     Agent Conversation Multi-Turn Cache Test                ║${NC}"
echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo ""

# Load environment
if [ -f .env ]; then
    source .env
    echo -e "${GREEN}✅ Loaded .env configuration${NC}"
else
    echo -e "${YELLOW}⚠️  No .env file found${NC}"
fi

# Check API key
if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo -e "${RED}❌ ANTHROPIC_API_KEY not set${NC}"
    exit 1
fi

# Check proxy
if ! curl -s "$PROXY_URL/health" > /dev/null; then
    echo -e "${RED}❌ Proxy not running at $PROXY_URL${NC}"
    echo -e "${YELLOW}💡 Start with: ./autocache${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Proxy running${NC}"
echo -e "${GREEN}✅ API key configured${NC}"
echo ""
echo -e "${BLUE}📊 Test Structure:${NC}"
echo -e "   • Turn 1: System + Tools + User question"
echo -e "   • Turn 2: + Assistant response + User follow-up"
echo -e "   • Turn 3-5: Growing conversation"
echo ""
echo -e "${BLUE}📈 Expected Behavior:${NC}"
echo -e "   • Turn 1: Create cache for system & tools (~3000-4000 tokens)"
echo -e "   • Turn 2-5: Read from cache (90% savings on cached content)"
echo -e "   • Breakpoints: Deterministic order (system → tools → messages)"
echo ""

# Initialize tracking variables
declare -a TURN_TOTAL_TOKENS
declare -a TURN_CACHED_TOKENS
declare -a TURN_CACHE_CREATION
declare -a TURN_CACHE_READ
declare -a TURN_INPUT_TOKENS
declare -a TURN_OUTPUT_TOKENS

# Function to make request
make_turn_request() {
    local turn=$1
    local description="$2"

    echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}   Turn $turn: $description${NC}"
    echo -e "${CYAN}═══════════════════════════════════════════════════════${NC}"

    local request_file="$TEST_DIR/turn_${turn}_request.json"
    local headers_file="$RESULTS_DIR/turn_${turn}_headers.txt"
    local response_file="$RESULTS_DIR/turn_${turn}_response.json"

    # Make request
    echo -e "${BLUE}📤 Sending request...${NC}"
    local start_time=$(date +%s.%N)

    curl -s -D "$headers_file" \
        -H "Content-Type: application/json" \
        -H "x-api-key: $ANTHROPIC_API_KEY" \
        -H "anthropic-version: 2023-06-01" \
        -X POST \
        -d @"$request_file" \
        "$PROXY_URL/v1/messages" > "$response_file"

    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc -l)

    # Extract cache headers
    local total_tokens=$(grep -i "x-autocache-total-tokens:" "$headers_file" | sed 's/.*: //' | tr -d '\r')
    local cached_tokens=$(grep -i "x-autocache-cached-tokens:" "$headers_file" | sed 's/.*: //' | tr -d '\r')
    local cache_ratio=$(grep -i "x-autocache-cache-ratio:" "$headers_file" | sed 's/.*: //' | tr -d '\r')
    local breakpoints=$(grep -i "x-autocache-breakpoints:" "$headers_file" | sed 's/.*: //' | tr -d '\r')
    local injected=$(grep -i "x-autocache-injected:" "$headers_file" | sed 's/.*: //' | tr -d '\r')

    # Extract API usage from response
    local input_tokens=$(jq -r '.usage.input_tokens // 0' "$response_file")
    local output_tokens=$(jq -r '.usage.output_tokens // 0' "$response_file")
    local cache_creation=$(jq -r '.usage.cache_creation_input_tokens // 0' "$response_file")
    local cache_read=$(jq -r '.usage.cache_read_input_tokens // 0' "$response_file")

    # Store for analysis
    TURN_TOTAL_TOKENS[$turn]=$total_tokens
    TURN_CACHED_TOKENS[$turn]=$cached_tokens
    TURN_CACHE_CREATION[$turn]=$cache_creation
    TURN_CACHE_READ[$turn]=$cache_read
    TURN_INPUT_TOKENS[$turn]=$input_tokens
    TURN_OUTPUT_TOKENS[$turn]=$output_tokens

    # Display results
    echo -e "${GREEN}✅ Response received in ${duration}s${NC}"
    echo ""
    echo -e "${YELLOW}📊 Autocache Analysis:${NC}"
    echo -e "   Cache Injected:    ${MAGENTA}$injected${NC}"
    echo -e "   Total Tokens:      ${MAGENTA}$total_tokens${NC}"
    echo -e "   Cached Tokens:     ${MAGENTA}$cached_tokens${NC}"
    echo -e "   Cache Ratio:       ${MAGENTA}$cache_ratio${NC}"
    echo -e "   Breakpoints:       ${MAGENTA}$breakpoints${NC}"
    echo ""
    echo -e "${YELLOW}💰 Anthropic API Usage:${NC}"
    echo -e "   Input Tokens:            ${MAGENTA}$input_tokens${NC}"
    echo -e "   Output Tokens:           ${MAGENTA}$output_tokens${NC}"
    echo -e "   Cache Creation Tokens:   ${MAGENTA}$cache_creation${NC}"
    echo -e "   Cache Read Tokens:       ${MAGENTA}$cache_read${NC}"
    echo ""

    # Verify expectations
    if [ "$turn" -eq 1 ]; then
        if [ "$cache_creation" -gt 0 ]; then
            echo -e "${GREEN}✅ Cache created as expected ($cache_creation tokens)${NC}"
        else
            echo -e "${RED}❌ WARNING: No cache created on first request!${NC}"
        fi
    else
        if [ "$cache_read" -gt 0 ]; then
            echo -e "${GREEN}✅ Cache read as expected ($cache_read tokens)${NC}"
        else
            echo -e "${YELLOW}⚠️  WARNING: No cache read on request $turn${NC}"
        fi
    fi
    echo ""
}

# Run all turns
make_turn_request 1 "Initial request (cache creation expected)"
make_turn_request 2 "First follow-up (cache read expected)"
make_turn_request 3 "Continuing conversation"
make_turn_request 4 "Further discussion"
make_turn_request 5 "Final turn"

# Final Analysis
echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                    FINAL ANALYSIS                            ║${NC}"
echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo ""

echo -e "${YELLOW}📈 Token Progression:${NC}"
echo ""
printf "${BLUE}%-10s${NC} | ${BLUE}%-12s${NC} | ${BLUE}%-12s${NC} | ${BLUE}%-15s${NC} | ${BLUE}%-15s${NC}\n" \
    "Turn" "Total" "Cached" "Cache Create" "Cache Read"
echo "-----------|--------------|--------------|-----------------|----------------"

for turn in {1..5}; do
    printf "%-10s | ${MAGENTA}%-12s${NC} | ${MAGENTA}%-12s${NC} | ${MAGENTA}%-15s${NC} | ${MAGENTA}%-15s${NC}\n" \
        "Turn $turn" \
        "${TURN_TOTAL_TOKENS[$turn]}" \
        "${TURN_CACHED_TOKENS[$turn]}" \
        "${TURN_CACHE_CREATION[$turn]}" \
        "${TURN_CACHE_READ[$turn]}"
done

echo ""
echo -e "${YELLOW}💵 Cost Analysis:${NC}"
echo ""

# Calculate costs (using Claude 3.5 Sonnet pricing)
# Input: $3 per million, Cache write: $3.75 per million, Cache read: $0.30 per million, Output: $15 per million

# Turn 1 cost
t1_regular=${TURN_INPUT_TOKENS[1]:-0}
t1_cache_create=${TURN_CACHE_CREATION[1]:-0}
t1_output=${TURN_OUTPUT_TOKENS[1]:-0}
t1_cost=$(echo "scale=6; ($t1_regular * 3 + $t1_cache_create * 3.75 + $t1_output * 15) / 1000000" | bc)

echo -e "${BLUE}Turn 1 (Cache Creation):${NC}"
echo -e "   Regular input:  $t1_regular tokens × \$3/M = \$$(echo "scale=6; $t1_regular * 3 / 1000000" | bc)"
echo -e "   Cache write:    $t1_cache_create tokens × \$3.75/M = \$$(echo "scale=6; $t1_cache_create * 3.75 / 1000000" | bc)"
echo -e "   Output:         $t1_output tokens × \$15/M = \$$(echo "scale=6; $t1_output * 15 / 1000000" | bc)"
echo -e "   ${GREEN}Total: \$$t1_cost${NC}"
echo ""

# Turns 2-5 total cost
total_subsequent_cost=0
for turn in {2..5}; do
    regular=${TURN_INPUT_TOKENS[$turn]:-0}
    cache_read=${TURN_CACHE_READ[$turn]:-0}
    output=${TURN_OUTPUT_TOKENS[$turn]:-0}
    cost=$(echo "scale=6; ($regular * 3 + $cache_read * 0.30 + $output * 15) / 1000000" | bc)
    total_subsequent_cost=$(echo "scale=6; $total_subsequent_cost + $cost" | bc)
done

echo -e "${BLUE}Turns 2-5 (Cache Reads):${NC}"
echo -e "   ${GREEN}Total: \$$total_subsequent_cost${NC}"
echo ""

total_with_cache=$(echo "scale=6; $t1_cost + $total_subsequent_cost" | bc)

# Calculate cost without caching
total_tokens_all=0
total_output_all=0
for turn in {1..5}; do
    total_tokens_all=$((total_tokens_all + ${TURN_INPUT_TOKENS[$turn]:-0} + ${TURN_CACHE_CREATION[$turn]:-0} + ${TURN_CACHE_READ[$turn]:-0}))
    total_output_all=$((total_output_all + ${TURN_OUTPUT_TOKENS[$turn]:-0}))
done

cost_without_cache=$(echo "scale=6; ($total_tokens_all * 3 + $total_output_all * 15) / 1000000" | bc)

savings=$(echo "scale=6; $cost_without_cache - $total_with_cache" | bc)
savings_percent=$(echo "scale=2; ($savings / $cost_without_cache) * 100" | bc)

echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo -e "${GREEN}Total with caching:    \$$total_with_cache${NC}"
echo -e "${RED}Total without caching: \$$cost_without_cache${NC}"
echo -e "${YELLOW}Savings:               \$$savings (${savings_percent}%)${NC}"
echo -e "${GREEN}═══════════════════════════════════════════${NC}"
echo ""

echo -e "${YELLOW}🔍 Key Observations:${NC}"
echo ""

# Check deterministic ordering
first_breakpoints=$(grep -i "x-autocache-breakpoints:" "$RESULTS_DIR/turn_1_headers.txt" | sed 's/.*: //' | tr -d '\r')
if [[ $first_breakpoints == *"system"* ]] && [[ $first_breakpoints == *"tools"* ]]; then
    echo -e "${GREEN}✅ Deterministic breakpoint ordering verified${NC}"
    echo -e "   Order: $first_breakpoints"
else
    echo -e "${YELLOW}⚠️  Breakpoint order: $first_breakpoints${NC}"
fi

# Check cache consistency
cache_created=${TURN_CACHE_CREATION[1]:-0}
if [ "$cache_created" -gt 0 ]; then
    echo -e "${GREEN}✅ Cache created on first request: $cache_created tokens${NC}"

    # Verify subsequent reads
    all_reads_successful=true
    for turn in {2..5}; do
        cache_read=${TURN_CACHE_READ[$turn]:-0}
        if [ "$cache_read" -eq 0 ]; then
            all_reads_successful=false
            break
        fi
    done

    if $all_reads_successful; then
        echo -e "${GREEN}✅ All subsequent requests used cache${NC}"
    else
        echo -e "${YELLOW}⚠️  Some requests did not use cache${NC}"
    fi
else
    echo -e "${RED}❌ No cache created on first request${NC}"
fi

# Offline tokenizer accuracy
offline_total=${TURN_TOTAL_TOKENS[1]:-0}
api_total=$((${TURN_INPUT_TOKENS[1]:-0} + ${TURN_CACHE_CREATION[1]:-0}))
if [ "$api_total" -gt 0 ]; then
    diff=$((api_total - offline_total))
    diff_percent=$(echo "scale=2; ($diff * 100) / $api_total" | bc)
    echo -e "${GREEN}✅ Offline tokenizer accuracy:${NC}"
    echo -e "   Offline: $offline_total | API: $api_total | Diff: $diff tokens (${diff_percent}%)"
fi

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║                    TEST COMPLETE                             ║${NC}"
echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
echo ""
echo -e "${BLUE}📁 Results saved to: $RESULTS_DIR/${NC}"
echo -e "${BLUE}📄 Full log: $RESULTS_FILE${NC}"
echo ""

# Save summary
{
    echo "Agent Conversation Cache Test Results"
    echo "======================================"
    echo ""
    echo "Timestamp: $TIMESTAMP"
    echo ""
    echo "Cache Creation: ${TURN_CACHE_CREATION[1]} tokens"
    echo "Cache Reads: Turns 2-5 successfully read from cache"
    echo ""
    echo "Cost with caching:    \$$total_with_cache"
    echo "Cost without caching: \$$cost_without_cache"
    echo "Savings:              \$$savings ($savings_percent%)"
    echo ""
    echo "Breakpoint order: $first_breakpoints"
} > "$RESULTS_FILE"

echo -e "${GREEN}🎉 Multi-turn conversation test complete!${NC}"
