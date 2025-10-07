#!/bin/bash

# Precision test runner for Autocache tokenizers
# Tests tokenizer accuracy against real Anthropic API

set -e

echo "======================================"
echo "Autocache Tokenizer Precision Tests"
echo "======================================"
echo ""

# Check for API key
if [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "⚠️  WARNING: ANTHROPIC_API_KEY not set"
    echo "   Precision tests that compare against real API will be skipped"
    echo "   Set ANTHROPIC_API_KEY to run full test suite"
    echo ""
fi

# Build
echo "📦 Building..."
go build -o autocache
echo "✓ Build successful"
echo ""

# Run quick tests (no API calls)
echo "🔬 Running Quick Tests (no API calls)..."
echo ""

echo "→ Tokenizer Consistency Tests..."
go test -v -run "TestTokenizerConsistency" -timeout 2m

echo ""
echo "→ Concurrent Tokenization Tests..."
go test -v -run "TestConcurrentTokenization" -timeout 2m

echo ""
echo "→ Tokenizer Comparison Tests..."
go test -v -run "TestTokenizerComparison" -timeout 2m

echo ""
echo "→ N8N Comparison Test..."
go test -v -run "TestTokenizerN8NComparison" -timeout 2m

echo ""
echo "→ Unicode Ordinal Indicators Tests..."
go test -v -run "TestUnicodeOrdinalIndicators" -timeout 2m

echo ""
echo "→ Unicode Accented Characters Tests..."
go test -v -run "TestUnicodeAccentedCharacters" -timeout 2m

echo ""
echo "→ Unicode Stress Tests..."
go test -v -run "TestUnicodeStressTest" -timeout 2m

echo ""
echo "→ Production Panic Scenario Tests..."
go test -v -run "TestProductionPanicScenarios" -timeout 2m

echo ""
echo "→ Concurrent Unicode Stress Tests..."
go test -v -run "TestConcurrentUnicodeStress" -timeout 5m

echo ""
echo "→ N8N Workflow Regression Test..."
go test -v -run "TestRegressionN8NWorkflow" -timeout 2m

# Run precision tests if API key available
if [ -n "$ANTHROPIC_API_KEY" ]; then
    echo ""
    echo "🎯 Running Precision Tests (with real API calls)..."
    echo "   Note: This will make actual API calls and incur costs"
    echo ""

    echo "→ Spanish Unicode Precision Test..."
    go test -v -run "TestPrecisionSpanishUnicode" -timeout 5m

    echo ""
    echo "→ N8N Workflow Precision Test..."
    go test -v -run "TestPrecisionN8NWorkflow" -timeout 5m

    echo ""
    echo "→ Edge Cases Precision Test..."
    go test -v -run "TestPrecisionEdgeCases" -timeout 10m

    echo ""
    echo "======================================"
    echo "✅ All tests passed!"
    echo "======================================"
else
    echo ""
    echo "======================================"
    echo "⚠️  Precision tests skipped (no API key)"
    echo "✅ Quick tests passed!"
    echo "======================================"
fi

echo ""
echo "Summary:"
echo "  - Tokenizer implementations tested: 3 (anthropic, offline, heuristic)"
echo "  - Unicode edge cases tested: ✓"
echo "  - Concurrent access tested: ✓"
echo "  - Production scenarios tested: ✓"
if [ -n "$ANTHROPIC_API_KEY" ]; then
    echo "  - API accuracy verified: ✓"
fi
echo ""
