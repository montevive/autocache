#!/bin/bash

# Autocache Test Runner
# Runs comprehensive tests for the autocache proxy

set -e

echo "🧪 Starting Autocache Test Suite"
echo "================================="

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Function to run tests with coverage
run_tests() {
    local test_name=$1
    local package=$2
    local pattern=$3

    print_status $BLUE "🔍 Running $test_name..."

    if [[ -n "$pattern" ]]; then
        go test -v -run "$pattern" -coverprofile="coverage_${test_name,,}.out" $package
    else
        go test -v -coverprofile="coverage_${test_name,,}.out" $package
    fi

    local exit_code=$?
    if [ $exit_code -eq 0 ]; then
        print_status $GREEN "✅ $test_name passed"
    else
        print_status $RED "❌ $test_name failed"
        return $exit_code
    fi
}

# Function to show coverage
show_coverage() {
    local test_name=$1
    local coverage_file="coverage_${test_name,,}.out"

    if [[ -f "$coverage_file" ]]; then
        local coverage=$(go tool cover -func="$coverage_file" | grep "total:" | awk '{print $3}')
        print_status $YELLOW "📊 $test_name coverage: $coverage"
    fi
}

# Clean up old coverage files
echo "🧹 Cleaning up old coverage files..."
rm -f coverage_*.out coverage.html

# Build the project first
print_status $BLUE "🔨 Building project..."
if go build -o autocache; then
    print_status $GREEN "✅ Build successful"
else
    print_status $RED "❌ Build failed"
    exit 1
fi

# Test categories
TESTS_PASSED=0
TESTS_FAILED=0

# Run unit tests for each component
echo ""
echo "📦 Running Unit Tests"
echo "===================="

# Tokenizer tests
if run_tests "Tokenizer Tests" "." "TestAnthropicTokenizer|TestCountTokens|TestGetModelMinimumTokens|TestIsCodeLike|TestIsJSONLike"; then
    ((TESTS_PASSED++))
    show_coverage "tokenizer_tests"
else
    ((TESTS_FAILED++))
fi

echo ""

# Pricing tests
if run_tests "Pricing Tests" "." "TestPricingCalculator|TestCalculateBaseCost|TestCalculateROI|TestFormatCost"; then
    ((TESTS_PASSED++))
    show_coverage "pricing_tests"
else
    ((TESTS_FAILED++))
fi

echo ""

# Cache injector tests
if run_tests "Cache Injector Tests" "." "TestCacheInjector|TestInjectCacheControl|TestCollectCacheCandidates"; then
    ((TESTS_PASSED++))
    show_coverage "cache_injector_tests"
else
    ((TESTS_FAILED++))
fi

echo ""

# Config tests
if run_tests "Config Tests" "." "TestLoadConfig|TestConfigValidation|TestGetEnv"; then
    ((TESTS_PASSED++))
    show_coverage "config_tests"
else
    ((TESTS_FAILED++))
fi

echo ""

# Handler integration tests
if run_tests "Handler Tests" "." "TestHandleMessages|TestHandleHealth|TestHandleMetrics"; then
    ((TESTS_PASSED++))
    show_coverage "handler_tests"
else
    ((TESTS_FAILED++))
fi

echo ""

# Run all tests together for overall coverage
echo "🎯 Running Complete Test Suite"
echo "============================="

if go test -v -coverprofile=coverage.out ./...; then
    print_status $GREEN "✅ All tests completed"

    # Generate HTML coverage report
    go tool cover -html=coverage.out -o coverage.html
    print_status $BLUE "📊 Coverage report generated: coverage.html"

    # Show overall coverage
    OVERALL_COVERAGE=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}')
    print_status $YELLOW "📈 Overall test coverage: $OVERALL_COVERAGE"

else
    print_status $RED "❌ Some tests failed"
    ((TESTS_FAILED++))
fi

echo ""

# Test summary
echo "📋 Test Summary"
echo "==============="
print_status $GREEN "✅ Test categories passed: $TESTS_PASSED"
if [[ $TESTS_FAILED -gt 0 ]]; then
    print_status $RED "❌ Test categories failed: $TESTS_FAILED"
else
    print_status $GREEN "🎉 All test categories passed!"
fi

echo ""

# Performance benchmarks (if available)
echo "⚡ Running Benchmarks"
echo "===================="

if go test -bench=. -benchmem -run=^$ . 2>/dev/null; then
    print_status $GREEN "✅ Benchmarks completed"
else
    print_status $YELLOW "⚠️  No benchmarks found or benchmarks failed"
fi

echo ""

# Race condition tests
echo "🏃 Running Race Detection Tests"
echo "==============================="

if go test -race -short .; then
    print_status $GREEN "✅ No race conditions detected"
else
    print_status $RED "❌ Race conditions detected"
    ((TESTS_FAILED++))
fi

echo ""

# Final summary
echo "🏁 Final Results"
echo "================"

if [[ $TESTS_FAILED -eq 0 ]]; then
    print_status $GREEN "🎉 All tests passed successfully!"
    print_status $BLUE "📊 Coverage report: file://$(pwd)/coverage.html"
    exit 0
else
    print_status $RED "❌ $TESTS_FAILED test categories failed"
    exit 1
fi