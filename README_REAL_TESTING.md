# Real API Testing Guide

This guide helps you perform real-world testing of the autocache proxy with actual Anthropic API calls.

## 🚀 Quick Start

### 1. Setup Your API Key

```bash
# Copy the template
cp .env.real .env

# Edit .env and add your real Anthropic API key
# ANTHROPIC_API_KEY=sk-ant-api03-your-actual-key-here
```

### 2. Start the Proxy

```bash
# Build if needed
go build -o autocache

# Start the proxy
./autocache
```

### 3. Run Real Tests

```bash
# Execute comprehensive real API tests
./test_real.sh
```

## 📊 What the Tests Do

### Test 1: Large System Prompt
- **Purpose**: Tests basic caching with a large system prompt (>1024 tokens)
- **Scenario**: Software engineering assistant with detailed instructions
- **Expected Result**:
  - First request: Cache creation, higher cost
  - Second request: Cache hit, ~90% cost reduction

### Test 2: System + Tools
- **Purpose**: Tests multiple cache breakpoints (system + tool definitions)
- **Scenario**: Assistant with calculator and weather tools
- **Expected Result**: Multiple cache points, optimal ROI

### Test 3: Strategy Comparison
- **Purpose**: Tests different caching strategies
- **Scenarios**: Conservative, Moderate, Aggressive strategies
- **Expected Result**: Different cache ratios and breakpoint counts

### Test 4: Cache Bypass
- **Purpose**: Verifies bypass functionality
- **Scenario**: Same large request with `X-Autocache-Bypass: true` header
- **Expected Result**: No caching, normal API behavior

## 📈 Interpreting Results

### Cache Headers to Monitor

```bash
X-Autocache-Injected: true/false          # Whether caching was applied
X-Autocache-Total-Tokens: 2048            # Total tokens in request
X-Autocache-Cached-Tokens: 1500           # Tokens that were cached
X-Autocache-Cache-Ratio: 0.732            # 73.2% of tokens cached
X-Autocache-ROI-Percent: 85.4             # 85.4% savings at scale
X-Autocache-ROI-BreakEven: 2              # Break-even at 2 requests
X-Autocache-ROI-Savings: $0.0156          # Savings per subsequent request
X-Autocache-Strategy: moderate            # Strategy used
X-Autocache-Breakpoints: system:1500:1h,tools:500:1h  # Cache breakpoints
```

### Anthropic API Usage

```json
{
  "usage": {
    "input_tokens": 500,                    // Non-cached tokens
    "output_tokens": 150,                   // Response tokens
    "cache_creation_input_tokens": 1500,    // Tokens written to cache (first request)
    "cache_read_input_tokens": 1500         // Tokens read from cache (subsequent requests)
  }
}
```

## 💰 Cost Analysis

### Example: Software Engineering Assistant

**Without Autocache:**
- Request cost: 2048 tokens × $3/1M = $0.006144

**With Autocache:**
- First request: 500 regular + 1500 cache write × 1.25 = $0.007125
- Subsequent requests: 500 regular + 1500 cache read × 0.1 = $0.00195
- **Savings per subsequent request: $0.004194 (68% reduction)**
- **Break-even: 1.3 requests**

### ROI Calculator

```bash
# After 10 requests:
# Without cache: 10 × $0.006144 = $0.06144
# With cache: $0.007125 + (9 × $0.00195) = $0.024675
# Total savings: $0.036765 (60% reduction)

# After 100 requests:
# Without cache: 100 × $0.006144 = $0.6144
# With cache: $0.007125 + (99 × $0.00195) = $0.200175
# Total savings: $0.414225 (67% reduction)
```

## 🔧 Advanced Testing

### Test Different Strategies

Restart the proxy with different strategies:

```bash
# Conservative (minimal caching)
CACHE_STRATEGY=conservative ./autocache

# Moderate (balanced)
CACHE_STRATEGY=moderate ./autocache

# Aggressive (maximum caching)
CACHE_STRATEGY=aggressive ./autocache
```

### Test Different Models

Edit test requests to use different models:

```json
{
  "model": "claude-3-haiku-20240307",    // Requires 2048+ tokens
  "model": "claude-3-5-sonnet-20241022", // Requires 1024+ tokens
  "model": "claude-3-opus-20240229"      // Requires 1024+ tokens
}
```

### Custom Test Scenarios

Create your own test scenarios:

```bash
# Create custom request
cat > my_test.json << 'EOF'
{
  "model": "claude-3-5-sonnet-20241022",
  "max_tokens": 500,
  "system": "Your large system prompt here...",
  "messages": [{"role": "user", "content": [{"type": "text", "text": "Your question"}]}]
}
EOF

# Test it
curl -H "Content-Type: application/json" \
     -H "Authorization: Bearer $ANTHROPIC_API_KEY" \
     -D headers.txt \
     -X POST \
     -d @my_test.json \
     http://localhost:8080/v1/messages
```

## 📊 Monitoring and Debugging

### Server Logs

Monitor autocache logs for cache decisions:

```bash
# Start with debug logging
LOG_LEVEL=debug ./autocache
```

Look for log entries:
```
INFO[2024-01-01T12:00:00Z] Cache injection completed cache_ratio=0.732 cached_tokens=1500 roi_percent=85.4
```

### Health Check

```bash
curl http://localhost:8080/health
```

### Metrics Endpoint

```bash
curl http://localhost:8080/metrics
```

## 🎯 Expected Performance

### Typical Cache Ratios
- **Conservative**: 40-60% (system + tools only)
- **Moderate**: 60-80% (system + tools + large content)
- **Aggressive**: 70-90% (maximum caching within limits)

### Typical Break-even Points
- **Large system prompts**: 1-2 requests
- **System + tools**: 1-2 requests
- **Multiple large blocks**: 2-3 requests

### Cost Savings
- **First request**: 10-25% additional cost (cache writes)
- **Subsequent requests**: 60-90% cost reduction
- **Overall after 10+ requests**: 50-80% total cost reduction

## 🚨 Troubleshooting

### Proxy Not Starting
```bash
# Check if port is in use
lsof -i :8080

# Check logs
LOG_LEVEL=debug ./autocache
```

### No Caching Applied
- Check token counts meet minimums (1024 for most models, 2048 for Haiku)
- Verify content is large enough
- Check strategy configuration

### API Key Issues
```bash
# Test direct API access
curl -H "Authorization: Bearer $ANTHROPIC_API_KEY" \
     https://api.anthropic.com/v1/messages
```

### Unexpected Costs
- Check cache ratios in headers
- Verify break-even calculations
- Monitor both cache writes and reads in usage

## 📁 Test Results

Test results are saved in `test_results/` directory:
- Request/response files
- Cache headers
- Detailed analysis logs
- Timestamped for easy comparison

## 🎉 Success Indicators

You're successfully using autocache when you see:
- ✅ `X-Autocache-Injected: true` in response headers
- ✅ Positive cache ratios (>0.5 for good content)
- ✅ Reasonable break-even points (1-3 requests)
- ✅ Actual cost reductions in Anthropic Console
- ✅ Faster subsequent requests (cache reads)