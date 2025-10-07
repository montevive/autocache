<div align="center">
  <img src="media/logo-autocache.png" alt="AutoCache Logo" width="400"/>

  # Autocache

  **Intelligent Anthropic API Cache Proxy with ROI Analytics**

  [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
  [![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev/)
  [![Tests](https://img.shields.io/badge/tests-passing-brightgreen)]()

</div>

Autocache is a smart proxy server that automatically injects cache-control fields into Anthropic Claude API requests, reducing costs by up to **90%** and latency by up to **85%** while providing detailed ROI analytics via response headers.

## Motivation

Modern AI agent platforms like **n8n**, **Flowise**, **Make.com**, and even popular frameworks like **LangChain** and **LlamaIndex** don't support Anthropic's prompt caching—despite users building increasingly complex agents with:

- 📝 **Large system prompts** (1,000-5,000+ tokens)
- 🛠️ **10+ tool definitions** (5,000-15,000+ tokens)
- 🔄 **Repeated agent interactions** (same context, different queries)

### The Problem

When you build a complex agent in **n8n** with a detailed system prompt and multiple tools, every API call sends the full context again—costing 10x more than necessary. For example:

- **Without caching**: 15,000 token agent → $0.045 per request
- **With caching**: Same agent → $0.0045 per request after first call (90% savings)

### Real User Pain Points

The AI community has been requesting this feature:

- 🔗 [n8n GitHub Issue #13231](https://github.com/n8n-io/n8n/issues/13231) - "Anthropic model not caching system prompt"
- 🔗 [Flowise Issue #4289](https://github.com/FlowiseAI/Flowise/issues/4289) - "Support for Anthropic Prompt Caching"
- 🔗 [n8n Community Request](https://community.n8n.io/t/request-prompt-caching-support-for-claude/101941) - Multiple requests for caching support
- 🔗 [LangChain Issue #26701](https://github.com/langchain-ai/langchain/issues/26701) - Implementation difficulties

### The Solution

**Autocache** works as a transparent proxy that automatically analyzes your requests and injects cache-control headers at optimal breakpoints—**no code changes required**. Just point your existing n8n/Flowise/Make.com workflows to Autocache instead of directly to Anthropic's API.

**Result**: Same agents, 90% lower costs, 85% lower latency—automatically.

## Alternatives & Comparison

Several tools offer prompt caching support, but Autocache is unique in combining **zero-config transparent proxy** with **intelligent ROI analytics**:

### Existing Solutions

| Solution | Type | Auto-Injection | Intelligence | ROI Analytics | Drop-in for n8n/Flowise |
|----------|------|----------------|--------------|---------------|-------------------------|
| **Autocache** | Proxy | ✅ Fully automatic | ✅ Token analysis + ROI scoring | ✅ Response headers | ✅ Yes |
| [LiteLLM](https://docs.litellm.ai/docs/tutorials/prompt_caching) | Proxy | ⚠️ Requires config | ❌ Rule-based | ❌ No | ✅ Yes |
| [langchain-smart-cache](https://github.com/imranarshad/langchain-anthropic-smart-cache) | Library | ✅ Fully automatic | ✅ Priority-based | ✅ Statistics | ❌ LangChain only |
| [anthropic-cost-tracker](https://github.com/Supgrade/anthropic-API-cost-tracker) | Library | ❓ Unclear | ❓ Unknown | ✅ Dashboard | ❌ Python only |
| OpenRouter | Service | ⚠️ Provider-dependent | ❌ No | ❌ No | ✅ Yes |
| AWS Bedrock | Cloud | ✅ ML-based | ✅ Yes | ✅ AWS only | ❌ AWS only |

### What Makes Autocache Different

**Autocache is the only solution that combines:**

1. 🔄 **Transparent Proxy** - Works with any tool (n8n, Flowise, Make.com) without code changes
2. 🧠 **Intelligent Analysis** - Automatic token counting, ROI scoring, and optimal breakpoint placement
3. 📊 **Real-time ROI** - Cost savings and break-even analysis in every response header
4. 🏠 **Self-Hosted** - No external dependencies or cloud vendor lock-in
5. ⚙️ **Zero Config** - Works out of the box with multiple strategies (conservative/moderate/aggressive)

**Other solutions** require configuration (LiteLLM), framework lock-in (langchain-smart-cache), or don't provide transparent proxy functionality for agent builders.

## Features

✨ **Drop-in Replacement**: Simply change your API URL and get automatic caching
📊 **ROI Analytics**: Detailed cost savings and break-even analysis via headers
🎯 **Smart Caching**: Intelligent placement of cache breakpoints using multiple strategies
⚡ **High Performance**: Supports both streaming and non-streaming requests
🔧 **Configurable**: Multiple caching strategies and customizable thresholds
🐳 **Docker Ready**: Easy deployment with Docker and docker-compose
📋 **Comprehensive Logging**: Detailed request/response logging with structured output

## Quick Start

### Using Docker Compose (Recommended)

1. **Clone and configure**:
```bash
git clone <repository-url>
cd autocache
cp .env.example .env
# Edit .env with your ANTHROPIC_API_KEY (optional - can pass in headers instead)
```

2. **Start the proxy**:
```bash
docker-compose up -d
```

3. **Use in your application**:
```bash
# Change your API base URL from:
# https://api.anthropic.com
# To:
# http://localhost:8080
```

### Direct Usage

1. **Build and run**:
```bash
go mod download
go build -o autocache ./cmd/autocache
# Option 1: Set API key via environment (optional)
ANTHROPIC_API_KEY=sk-ant-... ./autocache
# Option 2: Run without API key (pass it in request headers)
./autocache
```

2. **Configure your client**:
```python
# Python example - API key passed in headers
import anthropic

client = anthropic.Anthropic(
    api_key="sk-ant-...",  # This will be forwarded to Anthropic
    base_url="http://localhost:8080"  # Point to autocache
)
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `ANTHROPIC_API_KEY` | - | Your Anthropic API key (optional if passed in request headers) |
| `CACHE_STRATEGY` | `moderate` | Caching strategy: `conservative`/`moderate`/`aggressive` |
| `LOG_LEVEL` | `info` | Log level: `debug`/`info`/`warn`/`error` |
| `MAX_CACHE_BREAKPOINTS` | `4` | Maximum cache breakpoints (1-4) |
| `TOKEN_MULTIPLIER` | `1.0` | Token threshold multiplier |

### API Key Configuration

The Anthropic API key can be provided in three ways (in order of precedence):

1. **Request headers** (recommended for multi-tenant scenarios):
   ```http
   Authorization: Bearer sk-ant-...
   # or
   x-api-key: sk-ant-...
   ```

2. **Environment variable**:
   ```bash
   ANTHROPIC_API_KEY=sk-ant-... ./autocache
   ```

3. **`.env` file**:
   ```bash
   ANTHROPIC_API_KEY=sk-ant-...
   ```

💡 **Tip**: For multi-user environments (like n8n with multiple API keys), pass the key in request headers and leave the environment variable unset.

### Cache Strategies

#### 🛡️ Conservative
- **Focus**: System prompts and tools only
- **Breakpoints**: Maximum 2
- **Best For**: Cost-sensitive applications with predictable content

#### ⚖️ Moderate (Default)
- **Focus**: System, tools, and large content blocks
- **Breakpoints**: Maximum 3
- **Best For**: Most applications balancing savings and efficiency

#### 🚀 Aggressive
- **Focus**: Maximum caching coverage
- **Breakpoints**: All 4 available
- **Best For**: High-volume applications with repeated content

## ROI Analytics

Autocache provides detailed ROI metrics via response headers:

### Key Headers

| Header | Description |
|--------|-------------|
| `X-Autocache-Injected` | Whether caching was applied (`true`/`false`) |
| `X-Autocache-Cache-Ratio` | Percentage of tokens cached (0.0-1.0) |
| `X-Autocache-ROI-Percent` | Percentage savings at scale |
| `X-Autocache-ROI-BreakEven` | Requests needed to break even |
| `X-Autocache-Savings-100req` | Total savings after 100 requests |

### Example Response Headers

```http
X-Autocache-Injected: true
X-Autocache-Total-Tokens: 5120
X-Autocache-Cached-Tokens: 4096
X-Autocache-Cache-Ratio: 0.800
X-Autocache-ROI-FirstCost: $0.024
X-Autocache-ROI-Savings: $0.0184
X-Autocache-ROI-BreakEven: 2
X-Autocache-ROI-Percent: 85.2
X-Autocache-Breakpoints: system:2048:1h,tools:1024:1h,content:1024:5m
X-Autocache-Savings-100req: $1.75
```

## API Endpoints

### Main Endpoint
```
POST /v1/messages
```
Drop-in replacement for Anthropic's `/v1/messages` endpoint with automatic cache injection.

### Health Check
```
GET /health
```
Returns server health and configuration status.

### Metrics
```
GET /metrics
```
Returns supported models, strategies, and cache limits.

### Savings Analytics
```
GET /savings
```
Returns comprehensive ROI analytics and caching statistics:

**Response includes:**
- **Recent Requests**: Full history of recent requests with cache metadata
- **Aggregated Stats**:
  - Total requests processed
  - Requests with cache applied
  - Total tokens processed and cached
  - Average cache ratio
  - Projected savings after 10 and 100 requests
- **Debug Info**:
  - Breakpoints by type (system, tools, content)
  - Average tokens per breakpoint type
- **Configuration**: Current cache strategy and history size

**Example usage:**
```bash
curl http://localhost:8080/savings | jq '.aggregated_stats'
```

**Example response:**
```json
{
  "aggregated_stats": {
    "total_requests": 25,
    "requests_with_cache": 20,
    "total_tokens_processed": 125000,
    "total_tokens_cached": 95000,
    "average_cache_ratio": 0.76,
    "total_savings_after_10_reqs": "$1.85",
    "total_savings_after_100_reqs": "$18.50"
  },
  "debug_info": {
    "breakpoints_by_type": {
      "system": 15,
      "tools": 12,
      "content": 8
    },
    "average_tokens_by_type": {
      "system": 2048,
      "tools": 1536,
      "content": 1200
    }
  }
}
```

**Use cases:**
- 📊 Monitor cache effectiveness over time
- 🔍 Debug cache injection decisions
- 💰 Track actual cost savings
- 📈 Analyze which content types benefit most from caching

## Advanced Usage

### Bypass Caching
Add these headers to skip cache injection:
```http
X-Autocache-Bypass: true
# or
X-Autocache-Disable: true
```

### Custom Configuration
```bash
# Aggressive caching with debug logging
CACHE_STRATEGY=aggressive LOG_LEVEL=debug ./autocache

# Conservative caching with higher thresholds
CACHE_STRATEGY=conservative TOKEN_MULTIPLIER=1.5 ./autocache
```

### Production Deployment
```yaml
# docker-compose.prod.yml
version: '3.8'
services:
  autocache:
    image: autocache:latest
    environment:
      - LOG_JSON=true
      - LOG_LEVEL=info
      - CACHE_STRATEGY=aggressive
    ports:
      - "8080:8080"
    restart: unless-stopped
```

## How It Works

1. **Request Analysis**: Analyzes incoming Anthropic API requests
2. **Token Counting**: Uses approximated tokenization to identify cacheable content
3. **Smart Injection**: Places cache-control fields at optimal breakpoints:
   - System prompts (1h TTL)
   - Tool definitions (1h TTL)
   - Large content blocks (5m TTL)
4. **ROI Calculation**: Computes cost savings and break-even analysis
5. **Request Forwarding**: Sends enhanced request to Anthropic API
6. **Response Enhancement**: Adds ROI metadata to response headers

## Cache Control Details

### Supported Content Types
- ✅ System messages
- ✅ Tool definitions
- ✅ Text content blocks
- ✅ Message content
- ❌ Images (not cacheable per Anthropic limits)

### Token Requirements
- **Most models**: 1024 tokens minimum
- **Haiku models**: 2048 tokens minimum
- **Breakpoint limit**: 4 per request

### TTL Options
- **5 minutes**: Dynamic content, frequent changes
- **1 hour**: Stable content (system prompts, tools)

## Cost Savings Examples

### Example 1: Documentation Chat
```
Request: 8,000 tokens (6,000 cached system prompt + 2,000 user question)
Cost without caching: $0.024 per request
Cost with caching:
  - First request: $0.027 (includes cache write)
  - Subsequent requests: $0.0066 (90% savings)
  - Break-even: 2 requests
  - Savings after 100 requests: $1.62
```

### Example 2: Code Review Assistant
```
Request: 12,000 tokens (10,000 cached codebase + 2,000 review request)
Cost without caching: $0.036 per request
Cost with caching:
  - First request: $0.045 (includes cache write)
  - Subsequent requests: $0.009 (75% savings)
  - Break-even: 1 request
  - Savings after 100 requests: $2.61
```

## Monitoring and Debugging

### Logging
```bash
# Debug mode for detailed cache decisions
LOG_LEVEL=debug ./autocache

# JSON logging for production
LOG_JSON=true LOG_LEVEL=info ./autocache
```

### Key Log Fields
- `cache_injected`: Whether caching was applied
- `cache_ratio`: Percentage of tokens cached
- `breakpoints`: Number of cache breakpoints used
- `roi_percent`: Percentage savings achieved

### Health Monitoring
```bash
# Check proxy health
curl http://localhost:8080/health

# Get metrics and configuration
curl http://localhost:8080/metrics

# Get comprehensive savings analytics
curl http://localhost:8080/savings | jq .

# Monitor aggregated statistics
curl http://localhost:8080/savings | jq '.aggregated_stats'

# Check breakpoint distribution
curl http://localhost:8080/savings | jq '.debug_info.breakpoints_by_type'
```

## Troubleshooting

### Common Issues

**❌ No caching applied**
- Check token counts meet minimums (1024/2048)
- Verify content is cacheable (not images)
- Review cache strategy configuration

**❌ High break-even point**
- Content may be too small for effective caching
- Consider more conservative strategy
- Check token multiplier setting

**❌ API key errors**
- Ensure `ANTHROPIC_API_KEY` is set or passed in headers
- Verify API key format: `sk-ant-...`

### Debug Mode
```bash
LOG_LEVEL=debug ./autocache
```
Provides detailed information about:
- Token counting decisions
- Cache breakpoint placement
- ROI calculations
- Request/response processing

## Architecture

Autocache follows Go best practices with a clean, modular architecture:

```
autocache/
├── cmd/
│   └── autocache/           # Application entry point
├── internal/
│   ├── types/              # Shared data models
│   ├── config/             # Configuration management
│   ├── tokenizer/          # Token counting (heuristic, offline, API-based)
│   ├── pricing/            # Cost calculations and ROI
│   ├── client/             # Anthropic API client
│   ├── cache/              # Cache injection logic
│   └── server/             # HTTP handlers and routing
└── test_fixtures.go        # Shared test utilities
```

### Key Components

- **Server**: HTTP request handler with streaming support
- **Cache Injector**: Intelligent cache breakpoint placement with ROI scoring
- **Tokenizer**: Multiple implementations (heuristic, offline tokenizer, real API)
- **Pricing Calculator**: ROI and cost-benefit analysis
- **API Client**: Anthropic API communication with header management

For detailed architecture documentation, see [CLAUDE.md](CLAUDE.md).

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass: `go test ./...`
5. Submit a pull request

## License

MIT License - see LICENSE file for details.

## Support

- 📧 Email: support@autocache.example
- 💬 Issues: [GitHub Issues](https://github.com/yourusername/autocache/issues)
- 📖 Documentation: [GitHub Wiki](https://github.com/yourusername/autocache/wiki)

---

**Autocache** - Maximize your Anthropic API efficiency with intelligent caching 🚀