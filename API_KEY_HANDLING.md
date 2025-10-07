# API Key Handling in Autocache

## Overview

Autocache supports flexible API key configuration, allowing keys to be provided either in the proxy environment or in each request.

## Priority Order

The proxy checks for API keys in this order:

1. **Request Headers** (highest priority):
   - `Authorization: Bearer sk-ant-...`
   - `x-api-key: sk-ant-...`
   - `anthropic-api-key: sk-ant-...`

2. **Environment Variable** (fallback):
   - `ANTHROPIC_API_KEY=sk-ant-...`

## Implementation

See `proxy.go:217-241` for the `ExtractAPIKey` function:

```go
func ExtractAPIKey(headers http.Header) string {
    // Check Authorization header
    auth := headers.Get("Authorization")
    if auth != "" {
        if strings.HasPrefix(auth, "Bearer ") {
            return strings.TrimPrefix(auth, "Bearer ")
        }
        return auth
    }
    
    // Check x-api-key header
    apiKey := headers.Get("x-api-key")
    if apiKey != "" {
        return apiKey
    }
    
    // Check anthropic-api-key header
    // ... (fallback)
}
```

And `handler.go:254-264` for fallback logic:

```go
func (ah *AutocacheHandler) getAPIKey(r *http.Request) string {
    // First try to get from request headers
    apiKey := ExtractAPIKey(r.Header)
    if apiKey != "" {
        return apiKey
    }
    
    // Fall back to configured API key
    return ah.config.AnthropicAPIKey
}
```

## Usage Patterns

### Pattern 1: API Key in Requests (Recommended for n8n)

**Advantages**:
- Different users/workflows can use different API keys
- No need to restart proxy to change keys
- Better for multi-tenant scenarios

**Configuration**:
```yaml
# docker-compose.n8n.yml
autocache:
  environment:
    - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY:-}  # Optional fallback
```

**n8n HTTP Request Node**:
```
Headers:
  x-api-key: sk-ant-your-key-here
  anthropic-version: 2023-06-01
```

### Pattern 2: API Key in Proxy Environment

**Advantages**:
- Centralized key management
- No need to configure in each request
- Simpler for single-user scenarios

**Configuration**:
```yaml
# docker-compose.n8n.yml
autocache:
  environment:
    - ANTHROPIC_API_KEY=sk-ant-your-key-here
```

**n8n HTTP Request Node**:
```
Headers:
  anthropic-version: 2023-06-01
  # No API key needed - proxy provides it
```

### Pattern 3: Hybrid (Both)

**Use Case**: Provide a default key in environment, but allow override per request

**Configuration**:
```yaml
autocache:
  environment:
    - ANTHROPIC_API_KEY=sk-ant-default-key
```

**Request can override**:
```
Headers:
  x-api-key: sk-ant-custom-key  # This takes priority
```

## Security Notes

1. **Request headers are NOT logged** by default
2. API keys are redacted in logs: `***redacted***`
3. When forwarding to Anthropic, the key is sent in `x-api-key` header
4. Any `Authorization` header is removed before forwarding

## Testing

Test with curl:

```bash
# Using header (no .env needed)
curl -X POST http://localhost:8080/v1/messages \
  -H "x-api-key: sk-ant-your-key" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-3-5-sonnet-20241022","max_tokens":100,"messages":[{"role":"user","content":"Hello"}]}'

# Using environment variable (from proxy)
curl -X POST http://localhost:8080/v1/messages \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-3-5-sonnet-20241022","max_tokens":100,"messages":[{"role":"user","content":"Hello"}]}'
```

## Summary

✅ **API key in request headers** (recommended for n8n, multi-user)
✅ **API key in proxy environment** (simpler for single-user)
✅ **Both methods can coexist** (request header takes priority)
