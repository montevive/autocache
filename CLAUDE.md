# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Autocache is an intelligent proxy server for the Anthropic Claude API that automatically injects cache-control fields into requests to reduce costs by up to 90% and latency by up to 85%. It acts as a drop-in replacement proxy that intelligently places cache breakpoints based on token analysis and ROI calculations.

## Build and Test Commands

### Build
```bash
go build -o autocache ./cmd/autocache
```

### Run
```bash
# Start with defaults
./autocache

# With custom configuration
CACHE_STRATEGY=aggressive LOG_LEVEL=debug ./autocache

# Using Docker
docker-compose up -d
```

### Run Tests
```bash
# Run all unit tests
go test ./... -v

# Run unit tests with coverage
go test ./... -cover

# Run real API tests (requires ANTHROPIC_API_KEY in .env)
./test_real.sh

# Run specific test files
go test -v -run TestCacheInjector
```

### Development
```bash
# Run with debug logging
LOG_LEVEL=debug ./autocache

# Run with specific strategy
CACHE_STRATEGY=conservative ./autocache

# Check health
curl http://localhost:8080/health

# Check metrics
curl http://localhost:8080/metrics
```

## Architecture

### Project Structure

```
autocache-gh/
├── cmd/
│   └── autocache/
│       └── main.go                    # Entry point
├── internal/
│   ├── types/                         # Shared types and models
│   │   └── models.go
│   ├── config/                        # Configuration
│   │   └── config.go
│   ├── tokenizer/                     # Token counting
│   │   ├── tokenizer.go              # Interface + heuristic impl
│   │   ├── offline_tokenizer.go      # Embedded tokenizer
│   │   ├── real_tokenizer.go         # API-based tokenizer
│   │   └── anthropic_real_tokenizer.go
│   ├── pricing/                       # Cost calculations
│   │   └── pricing.go
│   ├── client/                        # Anthropic API client
│   │   └── anthropic.go
│   ├── cache/                         # Cache injection logic
│   │   └── injector.go
│   └── server/                        # HTTP server
│       └── handler.go
└── test_fixtures.go                   # Test helpers
```

### Core Components

**cmd/autocache/main.go** - Entry point, server setup, and graceful shutdown handling. Uses logrus for logging and handles signal termination.

**internal/server/handler.go** - HTTP request handler (`AutocacheHandler`) that orchestrates the complete request lifecycle:
- Routes: `/v1/messages` (main API), `/health`, `/metrics`
- Handles both streaming and non-streaming requests
- Adds cache metadata headers to responses (X-Autocache-* headers)
- Supports bypass headers (`X-Autocache-Bypass`, `X-Autocache-Disable`)

**internal/cache/injector.go** - Core caching logic (`CacheInjector`):
- Analyzes requests and identifies cacheable content blocks
- Creates `CacheCandidate` objects with ROI scores
- Sorts candidates by ROI and applies strategy limits
- Places `CacheControl` objects with type "ephemeral" and TTL ("5m" or "1h")
- Applies cache control to system prompts, tools, and content blocks

**internal/client/anthropic.go** - API communication (`ProxyClient`):
- Forwards requests to Anthropic API (default: https://api.anthropic.com)
- Handles both streaming and non-streaming responses
- Manages headers (authorization via x-api-key)
- Validates requests before forwarding

**internal/tokenizer/tokenizer.go** - Token estimation (`Tokenizer` interface):
- Approximates token counts using heuristics (1.5 chars/token base)
- Identifies code-like and JSON content for better estimates
- Returns model-specific minimums (1024 for most models, 2048 for Haiku)
- Multiple implementations: heuristic, offline (embedded), real (API-based)

**internal/pricing/pricing.go** - ROI calculation (`PricingCalculator`):
- Calculates cache write costs (1.25x multiplier) and read costs (0.1x multiplier)
- Computes break-even points and projected savings
- Maintains pricing for different Claude models

**internal/types/models.go** - Data structures:
- `AnthropicRequest/Response` - API request/response formats
- `CacheControl` - Cache control configuration (type: "ephemeral", ttl: "5m"/"1h")
- `CacheMetadata` - Metadata about caching decisions and ROI
- `CacheStrategy` - Conservative, Moderate, Aggressive configurations

**internal/config/config.go** - Configuration management:
- Loads from environment variables and .env file
- Validates configuration
- Sets up logger (text or JSON format)

### Request Flow

1. Client sends request to proxy at `/v1/messages`
2. `handler.go` receives request, validates, checks for bypass headers
3. `cache_injector.go` analyzes request:
   - Counts tokens in system prompts, tools, and content blocks
   - Identifies candidates meeting minimum token thresholds
   - Calculates ROI score for each candidate
   - Sorts by ROI and selects top N (based on strategy)
   - Injects `cache_control` fields at selected breakpoints
4. `proxy.go` forwards modified request to Anthropic API
5. Response is returned with X-Autocache-* headers containing ROI metrics

### Cache Strategies

**Conservative** (max 2 breakpoints):
- MinTokensMultiplier: 2.0 (strict requirements)
- Caches only system prompts and tools
- Best for cost-sensitive applications

**Moderate** (max 3 breakpoints, default):
- MinTokensMultiplier: 1.0 (standard requirements)
- Caches system, tools, and large content
- Balanced approach for most use cases

**Aggressive** (max 4 breakpoints):
- MinTokensMultiplier: 0.8 (lenient requirements)
- Maximum caching coverage
- Best for high-volume applications with repeated content

### Token Minimums

- Most Claude models: 1024 tokens minimum per breakpoint
- Haiku models (claude-3-haiku, claude-3-5-haiku): 2048 tokens minimum
- Maximum 4 cache breakpoints per request (Anthropic API limit)

## Important Implementation Details

### Cache Control Injection

Cache control is applied by adding a `cache_control` field to the last element in cacheable arrays:
- System blocks: Add to last ContentBlock in SystemBlocks array
- Tools: Add to last ToolDefinition in Tools array
- Message content: Add to last ContentBlock in Message.Content array

Format:
```json
{
  "cache_control": {
    "type": "ephemeral",
    "ttl": "1h"  // or "5m" for dynamic content
  }
}
```

### ROI Score Calculation

ROI score determines priority for cache breakpoint placement:
- Base score from savings potential (readSavings * 100)
- Bonuses: Large content (>2048 tokens: 1.2x, >5000: 1.3x)
- Content type multipliers: system (2.0x), tools (1.5x)
- Break-even bonuses: ≤2 requests (1.3x), ≤5 requests (1.1x)
- Penalties: >10 break-even (0.5x), >20 break-even (0.2x)

### API Key Handling

API keys can be provided in three ways (priority order):
1. Request headers: `Authorization: Bearer sk-ant-...`
2. Request headers: `x-api-key: sk-ant-...`
3. Environment variable: `ANTHROPIC_API_KEY`

The proxy converts all to `x-api-key` header when forwarding to Anthropic.

### Response Headers

All responses include X-Autocache-* headers:
- `X-Autocache-Injected`: Whether caching was applied
- `X-Autocache-Total-Tokens`: Total input tokens
- `X-Autocache-Cached-Tokens`: Tokens under cache control
- `X-Autocache-Cache-Ratio`: Ratio of cached tokens (0.0-1.0)
- `X-Autocache-ROI-*`: Cost and savings metrics
- `X-Autocache-Breakpoints`: Comma-separated list (position:tokens:ttl)

### Testing Considerations

- Unit tests use `AnthropicTokenizer` with approximations
- Real API tests require `ANTHROPIC_API_KEY` in environment
- Test fixtures in `test_fixtures.go` provide sample requests
- Files like `test_real.sh`, `test_large_content.sh` test with actual API
- Log files (proxy.log, proxy_debug.log) are generated during testing