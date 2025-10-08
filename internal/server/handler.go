package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"autocache/internal/cache"
	"autocache/internal/client"
	"autocache/internal/config"
	"autocache/internal/pricing"
	"autocache/internal/tokenizer"
	"autocache/internal/types"

	"github.com/sirupsen/logrus"
)

// AutocacheHandler handles HTTP requests and orchestrates cache injection
type AutocacheHandler struct {
	cacheInjector  *cache.CacheInjector
	proxyClient    *client.ProxyClient
	config         *config.Config
	logger         *logrus.Logger
	requestHistory []types.CacheMetadata
	historyMutex   sync.RWMutex
	panicCount     atomic.Uint64
	lastPanicTime  atomic.Int64
}

// NewAutocacheHandler creates a new handler
func NewAutocacheHandler(cfg *config.Config, logger *logrus.Logger) *AutocacheHandler {
	strategy := types.CacheStrategy(cfg.CacheStrategy)

	return &AutocacheHandler{
		cacheInjector:  cache.NewCacheInjectorWithConfig(strategy, cfg, logger),
		proxyClient:    client.NewProxyClient(cfg.AnthropicURL, logger),
		config:         cfg,
		logger:         logger,
		requestHistory: make([]types.CacheMetadata, 0, cfg.SavingsHistorySize),
	}
}

// storeRequestMetadata stores metadata for the savings endpoint (thread-safe)
func (ah *AutocacheHandler) storeRequestMetadata(metadata *types.CacheMetadata) {
	if ah.config.SavingsHistorySize == 0 {
		return // History disabled
	}

	ah.historyMutex.Lock()
	defer ah.historyMutex.Unlock()

	// Add to history
	ah.requestHistory = append(ah.requestHistory, *metadata)

	// Keep only the last N requests
	if len(ah.requestHistory) > ah.config.SavingsHistorySize {
		ah.requestHistory = ah.requestHistory[len(ah.requestHistory)-ah.config.SavingsHistorySize:]
	}
}

// HandleMessages handles POST /v1/messages requests
func (ah *AutocacheHandler) HandleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		ah.writeError(w, http.StatusMethodNotAllowed, "Only POST method is allowed")
		return
	}

	// Read and parse the request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		ah.logger.WithError(err).Error("Failed to read request body")
		ah.writeError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	var req types.AnthropicRequest
	if err := json.Unmarshal(body, &req); err != nil {
		ah.logger.WithError(err).Error("Failed to parse request JSON")
		ah.writeError(w, http.StatusBadRequest, "Invalid JSON in request body")
		return
	}

	// Validate the request
	if err := ah.proxyClient.ValidateRequest(&req); err != nil {
		ah.logger.WithError(err).Warn("Request validation failed")
		ah.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request: %s", err.Error()))
		return
	}

	// Log request summary
	ah.proxyClient.LogRequestSummary(&req)

	// Check if caching should be bypassed
	if ah.shouldBypassCaching(r) {
		ah.logger.Info("Bypassing cache injection due to header")
		ah.forwardWithoutCaching(w, r, &req)
		return
	}

	// Handle streaming vs non-streaming
	if client.IsStreamingRequest(&req) {
		ah.handleStreamingRequest(w, r, &req)
	} else {
		ah.handleNonStreamingRequest(w, r, &req)
	}
}

// handleNonStreamingRequest handles non-streaming requests with cache injection and metadata
func (ah *AutocacheHandler) handleNonStreamingRequest(w http.ResponseWriter, r *http.Request, req *types.AnthropicRequest) {
	// Inject cache control
	metadata, err := ah.cacheInjector.InjectCacheControl(req)
	if err != nil {
		ah.logger.WithError(err).Error("Failed to inject cache control")
		ah.writeError(w, http.StatusInternalServerError, "Failed to process cache injection")
		return
	}

	// Extract API key
	apiKey := ah.getAPIKey(r)
	headers := client.CreateHeadersMap(r.Header, apiKey, ah.logger)

	// Forward the request
	resp, err := ah.proxyClient.ForwardRequest(req, headers)
	if err != nil {
		ah.logger.WithError(err).Error("Failed to forward request")
		ah.writeError(w, http.StatusBadGateway, "Failed to forward request to Anthropic API")
		return
	}

	// Read and parse response
	_, responseBody, err := ah.proxyClient.ReadAndParseResponse(resp)
	if err != nil {
		ah.logger.WithError(err).Error("Failed to read response")
		// Forward the error response as-is
		ah.writeRawResponse(w, resp.StatusCode, responseBody, resp.Header)
		return
	}

	// Add cache metadata headers
	ah.addCacheMetadataHeaders(w, metadata)

	// Copy response headers from Anthropic (skip Content-Encoding as we may have decompressed)
	for key, values := range resp.Header {
		if key == "Content-Encoding" {
			continue // Skip - body may have been decompressed by proxy
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Write the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(responseBody)

	// Store metadata for savings endpoint
	ah.storeRequestMetadata(metadata)

	ah.logger.WithFields(logrus.Fields{
		"cache_injected": metadata.CacheInjected,
		"cache_ratio":    metadata.CacheRatio,
		"breakpoints":    len(metadata.Breakpoints),
		"roi_percent":    metadata.ROI.PercentSavings,
	}).Info("Successfully processed non-streaming request")
}

// handleStreamingRequest handles streaming requests with cache injection
func (ah *AutocacheHandler) handleStreamingRequest(w http.ResponseWriter, r *http.Request, req *types.AnthropicRequest) {
	// Inject cache control
	metadata, err := ah.cacheInjector.InjectCacheControl(req)
	if err != nil {
		ah.logger.WithError(err).Error("Failed to inject cache control")
		ah.writeError(w, http.StatusInternalServerError, "Failed to process cache injection")
		return
	}

	// Add cache metadata headers before streaming starts
	ah.addCacheMetadataHeaders(w, metadata)

	// Extract API key
	apiKey := ah.getAPIKey(r)
	headers := client.CreateHeadersMap(r.Header, apiKey, ah.logger)

	// Forward the streaming request
	err = ah.proxyClient.ForwardStreamingRequest(req, headers, w)
	if err != nil {
		ah.logger.WithError(err).Error("Failed to forward streaming request")
		// For streaming, we can't send a proper error response if streaming already started
		return
	}

	// Store metadata for savings endpoint
	ah.storeRequestMetadata(metadata)

	ah.logger.WithFields(logrus.Fields{
		"cache_injected": metadata.CacheInjected,
		"cache_ratio":    metadata.CacheRatio,
		"breakpoints":    len(metadata.Breakpoints),
		"streaming":      true,
	}).Info("Successfully processed streaming request")
}

// forwardWithoutCaching forwards the request without any cache injection
func (ah *AutocacheHandler) forwardWithoutCaching(w http.ResponseWriter, r *http.Request, req *types.AnthropicRequest) {
	// Set header to indicate caching was bypassed
	w.Header().Set("X-Autocache-Injected", "false")

	apiKey := ah.getAPIKey(r)
	headers := client.CreateHeadersMap(r.Header, apiKey, ah.logger)

	if client.IsStreamingRequest(req) {
		err := ah.proxyClient.ForwardStreamingRequest(req, headers, w)
		if err != nil {
			ah.logger.WithError(err).Error("Failed to forward request without caching")
		}
	} else {
		resp, err := ah.proxyClient.ForwardRequest(req, headers)
		if err != nil {
			ah.logger.WithError(err).Error("Failed to forward request without caching")
			ah.writeError(w, http.StatusBadGateway, "Failed to forward request")
			return
		}

		_, responseBody, err := ah.proxyClient.ReadAndParseResponse(resp)
		if err != nil {
			ah.writeRawResponse(w, resp.StatusCode, responseBody, resp.Header)
			return
		}

		// Copy response headers (skip Content-Encoding as we may have decompressed)
		for key, values := range resp.Header {
			if key == "Content-Encoding" {
				continue // Skip - body may have been decompressed by proxy
			}
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(responseBody)
	}
}

// addCacheMetadataHeaders adds cache metadata to response headers
func (ah *AutocacheHandler) addCacheMetadataHeaders(w http.ResponseWriter, metadata *types.CacheMetadata) {
	w.Header().Set("X-Autocache-Injected", strconv.FormatBool(metadata.CacheInjected))
	w.Header().Set("X-Autocache-Total-Tokens", strconv.Itoa(metadata.TotalTokens))
	w.Header().Set("X-Autocache-Cached-Tokens", strconv.Itoa(metadata.CachedTokens))
	w.Header().Set("X-Autocache-Cache-Ratio", fmt.Sprintf("%.3f", metadata.CacheRatio))
	w.Header().Set("X-Autocache-Strategy", metadata.Strategy)
	w.Header().Set("X-Autocache-Model", metadata.Model)

	// ROI headers
	w.Header().Set("X-Autocache-ROI-FirstCost", pricing.FormatCost(metadata.ROI.FirstRequestCost))
	w.Header().Set("X-Autocache-ROI-Savings", pricing.FormatCost(metadata.ROI.SubsequentSavings))
	w.Header().Set("X-Autocache-ROI-BreakEven", strconv.Itoa(metadata.ROI.BreakEvenRequests))
	w.Header().Set("X-Autocache-ROI-Percent", fmt.Sprintf("%.1f", metadata.ROI.PercentSavings))

	// Breakpoints header (compact format)
	if len(metadata.Breakpoints) > 0 {
		breakpointsStr := ""
		for i, bp := range metadata.Breakpoints {
			if i > 0 {
				breakpointsStr += ","
			}
			breakpointsStr += fmt.Sprintf("%s:%d:%s", bp.Position, bp.Tokens, bp.TTL)
		}
		w.Header().Set("X-Autocache-Breakpoints", breakpointsStr)
	}

	// Savings projections
	w.Header().Set("X-Autocache-Savings-10req", pricing.FormatCost(metadata.ROI.SavingsAt10Requests))
	w.Header().Set("X-Autocache-Savings-100req", pricing.FormatCost(metadata.ROI.SavingsAt100Requests))
}

// shouldBypassCaching checks if caching should be bypassed based on headers
func (ah *AutocacheHandler) shouldBypassCaching(r *http.Request) bool {
	// Check for bypass header
	bypass := r.Header.Get("X-Autocache-Bypass")
	if bypass == "true" || bypass == "1" {
		return true
	}

	// Check for disable header
	disable := r.Header.Get("X-Autocache-Disable")
	if disable == "true" || disable == "1" {
		return true
	}

	return false
}

// getAPIKey extracts API key from request or config
func (ah *AutocacheHandler) getAPIKey(r *http.Request) string {
	// First try to get from request headers
	apiKey := client.ExtractAPIKey(r.Header, ah.logger)
	if apiKey != "" {
		return apiKey
	}

	// Fall back to configured API key
	return ah.config.AnthropicAPIKey
}

// writeError writes an error response
func (ah *AutocacheHandler) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResp := map[string]interface{}{
		"error": map[string]interface{}{
			"type":    "autocache_error",
			"message": message,
		},
	}

	_ = json.NewEncoder(w).Encode(errorResp)
}

// writeRawResponse writes a raw response with headers
func (ah *AutocacheHandler) writeRawResponse(w http.ResponseWriter, statusCode int, body []byte, headers http.Header) {
	// Copy headers (skip Content-Encoding as we may have decompressed)
	for key, values := range headers {
		if key == "Content-Encoding" {
			continue // Skip - body may have been decompressed by proxy
		}
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

// HandleHealth handles health check requests
func (ah *AutocacheHandler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	health := map[string]interface{}{
		"status":   "healthy",
		"version":  "1.0.0",
		"strategy": ah.config.CacheStrategy,
	}

	_ = json.NewEncoder(w).Encode(health)
}

// HandleMetrics handles metrics endpoint
func (ah *AutocacheHandler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Get tokenizer panic stats if available
	tokenizerPanics := uint64(0)
	tokenizerFallbacks := uint64(0)
	if offlineTokenizer, ok := ah.cacheInjector.GetTokenizer().(*tokenizer.OfflineTokenizer); ok {
		stats := offlineTokenizer.GetPanicStats()
		if stats != nil {
			tokenizerPanics = stats["panic_count"]
			tokenizerFallbacks = stats["fallback_count"]
		}
	}

	// Get last panic time
	lastPanic := ah.lastPanicTime.Load()
	var lastPanicStr string
	if lastPanic > 0 {
		lastPanicStr = time.Unix(lastPanic, 0).UTC().Format(time.RFC3339)
	} else {
		lastPanicStr = "never"
	}

	metrics := map[string]interface{}{
		"supported_models": ah.cacheInjector.GetPricing().GetSupportedModels(),
		"strategies":       []string{"conservative", "moderate", "aggressive"},
		"cache_limits": map[string]interface{}{
			"max_breakpoints":     4,
			"min_tokens_default":  1024,
			"min_tokens_haiku":    2048,
			"ttl_options":         []string{"5m", "1h"},
		},
		"panic_recovery": map[string]interface{}{
			"http_panics_total":       ah.panicCount.Load(),
			"last_http_panic":         lastPanicStr,
			"tokenizer_panics_total":  tokenizerPanics,
			"tokenizer_fallback_used": tokenizerFallbacks,
		},
		"tokenizer": map[string]interface{}{
			"mode":         ah.config.TokenizerMode,
			"log_failures": ah.config.LogTokenizerFailures,
		},
	}

	_ = json.NewEncoder(w).Encode(metrics)
}

// HandleSavings handles the savings analytics endpoint
func (ah *AutocacheHandler) HandleSavings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Get copy of history (thread-safe)
	ah.historyMutex.RLock()
	history := make([]types.CacheMetadata, len(ah.requestHistory))
	copy(history, ah.requestHistory)
	ah.historyMutex.RUnlock()

	// Calculate aggregated statistics
	totalRequests := len(history)
	requestsWithCache := 0
	totalTokensProcessed := 0
	totalTokensCached := 0
	totalSavingsAt10 := 0.0
	totalSavingsAt100 := 0.0

	// Debug info: breakpoints by type
	breakpointsByType := map[string]int{
		"system":  0,
		"tools":   0,
		"content": 0,
	}
	tokensByType := map[string][]int{
		"system":  {},
		"tools":   {},
		"content": {},
	}

	for _, meta := range history {
		totalTokensProcessed += meta.TotalTokens
		totalTokensCached += meta.CachedTokens

		if meta.CacheInjected {
			requestsWithCache++
			totalSavingsAt10 += meta.ROI.SavingsAt10Requests
			totalSavingsAt100 += meta.ROI.SavingsAt100Requests
		}

		// Count breakpoints by type
		for _, bp := range meta.Breakpoints {
			breakpointsByType[bp.Type]++
			tokensByType[bp.Type] = append(tokensByType[bp.Type], bp.Tokens)
		}
	}

	// Calculate average cache ratio
	avgCacheRatio := 0.0
	if totalTokensProcessed > 0 {
		avgCacheRatio = float64(totalTokensCached) / float64(totalTokensProcessed)
	}

	// Calculate average tokens by type
	avgTokensByType := map[string]int{}
	for typeName, tokens := range tokensByType {
		if len(tokens) > 0 {
			sum := 0
			for _, t := range tokens {
				sum += t
			}
			avgTokensByType[typeName] = sum / len(tokens)
		} else {
			avgTokensByType[typeName] = 0
		}
	}

	// Build response
	response := map[string]interface{}{
		"recent_requests": history,
		"aggregated_stats": map[string]interface{}{
			"total_requests":         totalRequests,
			"requests_with_cache":    requestsWithCache,
			"total_tokens_processed": totalTokensProcessed,
			"total_tokens_cached":    totalTokensCached,
			"average_cache_ratio":    avgCacheRatio,
			"total_savings_after_10_reqs":  pricing.FormatCost(totalSavingsAt10),
			"total_savings_after_100_reqs": pricing.FormatCost(totalSavingsAt100),
		},
		"debug_info": map[string]interface{}{
			"breakpoints_by_type":   breakpointsByType,
			"average_tokens_by_type": avgTokensByType,
		},
		"config": map[string]interface{}{
			"history_size": ah.config.SavingsHistorySize,
			"strategy":     ah.config.CacheStrategy,
		},
	}

	_ = json.NewEncoder(w).Encode(response)
}

// SetupRoutes sets up HTTP routes
func (ah *AutocacheHandler) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// Main API endpoint
	mux.HandleFunc("/v1/messages", ah.HandleMessages)

	// Health check
	mux.HandleFunc("/health", ah.HandleHealth)
	mux.HandleFunc("/", ah.HandleHealth) // Root also serves health

	// Metrics and analytics
	mux.HandleFunc("/metrics", ah.HandleMetrics)
	mux.HandleFunc("/savings", ah.HandleSavings)

	return mux
}

// LogMiddleware provides request logging
func (ah *AutocacheHandler) LogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response wrapper to capture status code
		wrapper := &responseWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		// Log request
		ah.logger.WithFields(logrus.Fields{
			"method":     r.Method,
			"url":        r.URL.Path,
			"user_agent": r.Header.Get("User-Agent"),
			"remote_addr": getClientIP(r),
		}).Info("Request started")

		// Call next handler
		next.ServeHTTP(wrapper, r)

		// Log response
		duration := time.Since(start)
		ah.logger.WithFields(logrus.Fields{
			"method":      r.Method,
			"url":         r.URL.Path,
			"status_code": wrapper.statusCode,
			"duration_ms": duration.Milliseconds(),
		}).Info("Request completed")
	})
}

// PanicRecoveryMiddleware recovers from panics in HTTP handlers
func (ah *AutocacheHandler) PanicRecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// Increment panic counter
				ah.panicCount.Add(1)
				ah.lastPanicTime.Store(time.Now().Unix())

				// Capture stack trace
				stack := debug.Stack()

				// Log the panic with full details
				ah.logger.WithFields(logrus.Fields{
					"panic_value":   fmt.Sprintf("%v", rec),
					"method":        r.Method,
					"url":           r.URL.Path,
					"remote_addr":   getClientIP(r),
					"user_agent":    r.Header.Get("User-Agent"),
					"total_panics":  ah.panicCount.Load(),
					"stack_trace":   string(stack),
				}).Error("Panic recovered in HTTP handler")

				// Return error response to client
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)

				errorResp := map[string]interface{}{
					"error": map[string]interface{}{
						"type":    "internal_server_error",
						"message": "An unexpected error occurred. Please try again.",
					},
				}

				_ = json.NewEncoder(w).Encode(errorResp)
			}
		}()

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// responseWrapper wraps http.ResponseWriter to capture status code
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// getClientIP gets the client IP address from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// Take the first IP
		ips := strings.Split(forwarded, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}