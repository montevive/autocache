package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"autocache/internal/config"
	"autocache/internal/types"

	"github.com/sirupsen/logrus"
)


// MockAnthropicServer creates a mock Anthropic API server for testing
func createMockAnthropicServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Read and parse the request
		var req types.AnthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"type":    "invalid_request_error",
					"message": "Invalid JSON",
				},
			})
			return
		}

		// Mock response
		response := types.AnthropicResponse{
			ID:         "msg_test_123",
			Type:       "message",
			Role:       "assistant",
			Model:      req.Model,
			StopReason: "end_turn",
			Content: []types.ContentBlock{
				{
					Type: "text",
					Text: "This is a mock response from the Anthropic API.",
				},
			},
			Usage: types.Usage{
				InputTokens:  1000,
				OutputTokens: 50,
			},
		}

		// If request has cache control, simulate cache usage
		hasCacheControl := false
		cacheTokens := 0

		// Check system message for cache control
		if req.System != "" {
			cacheTokens += 500 // Mock cached system tokens
			hasCacheControl = true
		}

		// Check tools for cache control
		for _, tool := range req.Tools {
			if tool.CacheControl != nil {
				cacheTokens += 200 // Mock cached tool tokens
				hasCacheControl = true
			}
		}

		// Check message content for cache control
		for _, msg := range req.Messages {
			for _, block := range msg.Content {
				if block.CacheControl != nil {
					cacheTokens += 300 // Mock cached content tokens
					hasCacheControl = true
				}
			}
		}

		if hasCacheControl {
			response.Usage.CacheCreationInputTokens = cacheTokens
			response.Usage.InputTokens -= cacheTokens
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	}))
}

func TestNewAutocacheHandler(t *testing.T) {
	cfg := &config.Config{
		AnthropicURL:  "https://api.anthropic.com",
		CacheStrategy: "moderate",
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewAutocacheHandler(cfg, logger)
	if handler == nil {
		t.Fatal("Expected handler to be created")
	}
	if handler.config != cfg {
		t.Error("Handler config not set correctly")
	}
	if handler.cacheInjector == nil {
		t.Error("Cache injector not initialized")
	}
	if handler.proxyClient == nil {
		t.Error("Proxy client not initialized")
	}
}

func TestHandleMessages(t *testing.T) {
	// Create mock Anthropic server
	mockServer := createMockAnthropicServer()
	defer mockServer.Close()

	// Create handler with mock server URL
	cfg := &config.Config{
		AnthropicURL:    mockServer.URL,
		AnthropicAPIKey: "sk-ant-test",
		CacheStrategy:   "moderate",
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewAutocacheHandler(cfg, logger)

	tests := []struct {
		name           string
		method         string
		request        *types.AnthropicRequest
		expectStatus   int
		expectCached   bool
		expectHeaders  []string
	}{
		{
			name:   "Valid request with cacheable content",
			method: "POST",
			request: &types.AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 100,
				System:    strings.Repeat("You are a helpful assistant. ", 100), // Large system prompt
				Messages: []types.Message{
					{
						Role: "user",
						Content: []types.ContentBlock{
							{Type: "text", Text: "Hello, how are you?"},
						},
					},
				},
			},
			expectStatus: http.StatusOK,
			expectCached: true,
			expectHeaders: []string{
				"X-Autocache-Injected",
				"X-Autocache-Total-Tokens",
				"X-Autocache-Cached-Tokens",
				"X-Autocache-ROI-Percent",
			},
		},
		{
			name:   "Request with small content (no caching)",
			method: "POST",
			request: &types.AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 100,
				System:    "Short system.", // Too small to cache
				Messages: []types.Message{
					{
						Role: "user",
						Content: []types.ContentBlock{
							{Type: "text", Text: "Hello"},
						},
					},
				},
			},
			expectStatus: http.StatusOK,
			expectCached: false,
			expectHeaders: []string{
				"X-Autocache-Injected",
			},
		},
		{
			name:   "Request with tools and system prompt",
			method: "POST",
			request: &types.AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 100,
				System:    strings.Repeat("System instructions. ", 100),
				Tools: []types.ToolDefinition{
					{
						Name:        "calculator",
						Description: strings.Repeat("A calculator tool. ", 50),
					},
				},
				Messages: []types.Message{
					{
						Role: "user",
						Content: []types.ContentBlock{
							{Type: "text", Text: "Calculate 2+2"},
						},
					},
				},
			},
			expectStatus: http.StatusOK,
			expectCached: true,
			expectHeaders: []string{
				"X-Autocache-Injected",
				"X-Autocache-Breakpoints",
				"X-Autocache-ROI-BreakEven",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create request
			reqBody, _ := json.Marshal(tt.request)
			req := httptest.NewRequest(tt.method, "/v1/messages", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+cfg.AnthropicAPIKey)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Call handler
			handler.HandleMessages(rr, req)

			// Check status code
			if rr.Code != tt.expectStatus {
				t.Errorf("Expected status %d, got %d", tt.expectStatus, rr.Code)
			}

			// Check headers
			for _, headerName := range tt.expectHeaders {
				if rr.Header().Get(headerName) == "" {
					t.Errorf("Expected header %s to be present", headerName)
				}
			}

			// Check cache injection header
			cacheInjected := rr.Header().Get("X-Autocache-Injected")
			expectedInjected := "false"
			if tt.expectCached {
				expectedInjected = "true"
			}
			if cacheInjected != expectedInjected {
				t.Errorf("Expected X-Autocache-Injected=%s, got %s", expectedInjected, cacheInjected)
			}

			// If response should be successful, check response body
			if tt.expectStatus == http.StatusOK {
				var response types.AnthropicResponse
				if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
					t.Errorf("Failed to parse response JSON: %v", err)
				}
				if response.ID == "" {
					t.Error("Expected response to have an ID")
				}
			}
		})
	}
}

func TestHandleMessagesInvalidRequests(t *testing.T) {
	cfg := &config.Config{
		AnthropicURL:    "https://api.anthropic.com",
		AnthropicAPIKey: "sk-ant-test",
		CacheStrategy:   "moderate",
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewAutocacheHandler(cfg, logger)

	tests := []struct {
		name         string
		method       string
		body         string
		expectStatus int
	}{
		{
			name:         "Wrong HTTP method",
			method:       "GET",
			body:         `{}`,
			expectStatus: http.StatusMethodNotAllowed,
		},
		{
			name:         "Invalid JSON",
			method:       "POST",
			body:         `{invalid json`,
			expectStatus: http.StatusBadRequest,
		},
		{
			name:         "Missing model",
			method:       "POST",
			body:         `{"max_tokens": 100, "messages": []}`,
			expectStatus: http.StatusBadRequest,
		},
		{
			name:         "Invalid max_tokens",
			method:       "POST",
			body:         `{"model": "claude-3-5-sonnet-20241022", "max_tokens": -1, "messages": []}`,
			expectStatus: http.StatusBadRequest,
		},
		{
			name:         "No messages",
			method:       "POST",
			body:         `{"model": "claude-3-5-sonnet-20241022", "max_tokens": 100, "messages": []}`,
			expectStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/v1/messages", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.HandleMessages(rr, req)

			if rr.Code != tt.expectStatus {
				t.Errorf("Expected status %d, got %d", tt.expectStatus, rr.Code)
			}
		})
	}
}

func TestHandleMessagesBypass(t *testing.T) {
	mockServer := createMockAnthropicServer()
	defer mockServer.Close()

	cfg := &config.Config{
		AnthropicURL:    mockServer.URL,
		AnthropicAPIKey: "sk-ant-test",
		CacheStrategy:   "moderate",
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewAutocacheHandler(cfg, logger)

	request := &types.AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 100,
		System:    strings.Repeat("System prompt with detailed instructions and comprehensive context. ", 150), // ~9000 chars = ~6000 tokens (would be cached)
		Messages: []types.Message{
			{
				Role: "user",
				Content: []types.ContentBlock{
					{Type: "text", Text: "Hello"},
				},
			},
		},
	}

	tests := []struct {
		name         string
		bypassHeader string
		expectCached bool
	}{
		{
			name:         "Normal request (should cache)",
			bypassHeader: "",
			expectCached: true,
		},
		{
			name:         "Bypass with X-Autocache-Bypass",
			bypassHeader: "X-Autocache-Bypass",
			expectCached: false,
		},
		{
			name:         "Bypass with X-Autocache-Disable",
			bypassHeader: "X-Autocache-Disable",
			expectCached: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody, _ := json.Marshal(request)
			req := httptest.NewRequest("POST", "/v1/messages", bytes.NewBuffer(reqBody))
			req.Header.Set("Content-Type", "application/json")

			if tt.bypassHeader != "" {
				req.Header.Set(tt.bypassHeader, "true")
			}

			rr := httptest.NewRecorder()
			handler.HandleMessages(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", rr.Code)
			}

			cacheInjected := rr.Header().Get("X-Autocache-Injected")
			expectedInjected := "false"
			if tt.expectCached {
				expectedInjected = "true"
			}

			if cacheInjected != expectedInjected {
				t.Errorf("Expected X-Autocache-Injected=%s, got %s", expectedInjected, cacheInjected)
			}
		})
	}
}

func TestHandleHealth(t *testing.T) {
	cfg := &config.Config{CacheStrategy: "moderate"}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewAutocacheHandler(cfg, logger)

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	handler.HandleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Error("Expected Content-Type to be application/json")
	}

	var health map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &health); err != nil {
		t.Errorf("Failed to parse health response: %v", err)
	}

	if health["status"] != "healthy" {
		t.Errorf("Expected status=healthy, got %v", health["status"])
	}

	if health["strategy"] != "moderate" {
		t.Errorf("Expected strategy=moderate, got %v", health["strategy"])
	}
}

func TestHandleMetrics(t *testing.T) {
	cfg := &config.Config{CacheStrategy: "aggressive"}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewAutocacheHandler(cfg, logger)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()

	handler.HandleMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var metrics map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &metrics); err != nil {
		t.Errorf("Failed to parse metrics response: %v", err)
	}

	// Check expected fields
	expectedFields := []string{"supported_models", "strategies", "cache_limits"}
	for _, field := range expectedFields {
		if _, exists := metrics[field]; !exists {
			t.Errorf("Expected field %s to be present in metrics", field)
		}
	}

	// Check supported models
	supportedModels, ok := metrics["supported_models"].([]interface{})
	if !ok || len(supportedModels) == 0 {
		t.Error("Expected supported_models to be a non-empty array")
	}

	// Check cache limits
	cacheLimits, ok := metrics["cache_limits"].(map[string]interface{})
	if !ok {
		t.Error("Expected cache_limits to be an object")
	} else {
		if cacheLimits["max_breakpoints"] != float64(4) {
			t.Errorf("Expected max_breakpoints=4, got %v", cacheLimits["max_breakpoints"])
		}
	}
}

func TestSetupRoutes(t *testing.T) {
	cfg := &config.Config{CacheStrategy: "moderate"}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewAutocacheHandler(cfg, logger)
	mux := handler.SetupRoutes()

	if mux == nil {
		t.Fatal("Expected mux to be created")
	}

	// Test route registration by making requests
	testRoutes := []struct {
		path           string
		expectStatus   int
		expectNotFound bool
	}{
		{"/health", http.StatusOK, false},
		{"/", http.StatusOK, false}, // Root should serve health
		{"/metrics", http.StatusOK, false},
		{"/v1/messages", http.StatusMethodNotAllowed, false}, // POST only
		{"/nonexistent", http.StatusOK, false}, // Caught by "/" handler (serves health)
	}

	for _, tt := range testRoutes {
		t.Run("Route: "+tt.path, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()

			mux.ServeHTTP(rr, req)

			if tt.expectNotFound && rr.Code != http.StatusNotFound {
				t.Errorf("Expected 404 for %s, got %d", tt.path, rr.Code)
			} else if !tt.expectNotFound && rr.Code != tt.expectStatus {
				t.Errorf("Expected %d for %s, got %d", tt.expectStatus, tt.path, rr.Code)
			}
		})
	}
}

func TestLogMiddleware(t *testing.T) {
	cfg := &config.Config{CacheStrategy: "moderate"}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Suppress logs during test

	handler := NewAutocacheHandler(cfg, logger)

	// Create a simple test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap with logging middleware
	wrappedHandler := handler.LogMiddleware(testHandler)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "127.0.0.1:12345"

	rr := httptest.NewRecorder()
	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got %s", rr.Body.String())
	}
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name           string
		setupRequest   func(*http.Request)
		expectedPrefix string
	}{
		{
			name: "X-Forwarded-For header",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "192.168.1.1, 10.0.0.1")
			},
			expectedPrefix: "192.168.1.1",
		},
		{
			name: "X-Real-IP header",
			setupRequest: func(r *http.Request) {
				r.Header.Set("X-Real-IP", "192.168.1.2")
			},
			expectedPrefix: "192.168.1.2",
		},
		{
			name: "RemoteAddr fallback",
			setupRequest: func(r *http.Request) {
				r.RemoteAddr = "127.0.0.1:12345"
			},
			expectedPrefix: "127.0.0.1:12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			tt.setupRequest(req)

			ip := getClientIP(req)
			if !strings.HasPrefix(ip, tt.expectedPrefix) {
				t.Errorf("Expected IP to start with %s, got %s", tt.expectedPrefix, ip)
			}
		})
	}
}

func TestResponseWrapper(t *testing.T) {
	rr := httptest.NewRecorder()
	wrapper := &responseWrapper{ResponseWriter: rr, statusCode: http.StatusOK}

	// Test default status code
	if wrapper.statusCode != http.StatusOK {
		t.Errorf("Expected default status code 200, got %d", wrapper.statusCode)
	}

	// Test WriteHeader
	wrapper.WriteHeader(http.StatusNotFound)
	if wrapper.statusCode != http.StatusNotFound {
		t.Errorf("Expected status code 404 after WriteHeader, got %d", wrapper.statusCode)
	}

	// Test that underlying ResponseWriter receives the status
	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected underlying ResponseWriter to have status 404, got %d", rr.Code)
	}
}

func TestWriteError(t *testing.T) {
	cfg := &config.Config{CacheStrategy: "moderate"}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewAutocacheHandler(cfg, logger)

	rr := httptest.NewRecorder()
	handler.writeError(rr, http.StatusBadRequest, "Test error message")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Error("Expected Content-Type to be application/json")
	}

	var errorResp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &errorResp); err != nil {
		t.Errorf("Failed to parse error response: %v", err)
	}

	errorObj, ok := errorResp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}

	if errorObj["type"] != "autocache_error" {
		t.Errorf("Expected error type 'autocache_error', got %v", errorObj["type"])
	}

	if errorObj["message"] != "Test error message" {
		t.Errorf("Expected error message 'Test error message', got %v", errorObj["message"])
	}
}

// TestPanicRecoveryMiddleware tests the panic recovery middleware
func TestPanicRecoveryMiddleware(t *testing.T) {
	cfg := &config.Config{CacheStrategy: "moderate"}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Suppress error logs during test

	handler := NewAutocacheHandler(cfg, logger)

	t.Run("Handler that panics", func(t *testing.T) {
		// Create a handler that deliberately panics
		panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("deliberate test panic")
		})

		// Wrap with panic recovery middleware
		wrapped := handler.PanicRecoveryMiddleware(panicHandler)

		// Create test request
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		// Execute - should not panic
		wrapped.ServeHTTP(rr, req)

		// Verify we got 500 error response
		if rr.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", rr.Code)
		}

		// Verify response is JSON
		if rr.Header().Get("Content-Type") != "application/json" {
			t.Error("Expected Content-Type to be application/json")
		}

		// Parse response
		var errorResp map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &errorResp); err != nil {
			t.Errorf("Failed to parse error response: %v", err)
		}

		// Verify error structure
		errorObj, ok := errorResp["error"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected error object in response")
		}

		if errorObj["type"] != "internal_server_error" {
			t.Errorf("Expected error type 'internal_server_error', got %v", errorObj["type"])
		}
	})

	t.Run("Handler that succeeds", func(t *testing.T) {
		// Create a normal handler
		normalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
		})

		// Wrap with panic recovery middleware
		wrapped := handler.PanicRecoveryMiddleware(normalHandler)

		// Create test request
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()

		// Execute
		wrapped.ServeHTTP(rr, req)

		// Verify normal response
		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", rr.Code)
		}

		if rr.Body.String() != "success" {
			t.Errorf("Expected body 'success', got %q", rr.Body.String())
		}
	})
}

// TestPanicRecoveryMetrics tests panic metrics tracking
func TestPanicRecoveryMetrics(t *testing.T) {
	cfg := &config.Config{
		CacheStrategy: "moderate",
		EnableMetrics: true,
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewAutocacheHandler(cfg, logger)

	// Check initial panic count
	initialPanics := handler.panicCount.Load()

	// Create a handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic for metrics")
	})

	wrapped := handler.PanicRecoveryMiddleware(panicHandler)

	// Trigger panic
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	// Check panic count increased
	newPanics := handler.panicCount.Load()
	if newPanics != initialPanics+1 {
		t.Errorf("Expected panic count to increase from %d to %d, got %d",
			initialPanics, initialPanics+1, newPanics)
	}

	// Check last panic time was set
	lastPanic := handler.lastPanicTime.Load()
	if lastPanic == 0 {
		t.Error("Expected lastPanicTime to be set")
	}
}

// TestPanicRecoveryInMetricsEndpoint tests panic metrics in /metrics endpoint
func TestPanicRecoveryInMetricsEndpoint(t *testing.T) {
	cfg := &config.Config{
		CacheStrategy: "moderate",
		EnableMetrics: true,
	}
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewAutocacheHandler(cfg, logger)

	// Trigger a panic first
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	wrapped := handler.PanicRecoveryMiddleware(panicHandler)
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	// Now check metrics endpoint
	req = httptest.NewRequest("GET", "/metrics", nil)
	rr = httptest.NewRecorder()

	handler.HandleMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var metrics map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &metrics); err != nil {
		t.Fatalf("Failed to parse metrics response: %v", err)
	}

	// Check panic_recovery section exists
	panicRecovery, ok := metrics["panic_recovery"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected panic_recovery in metrics")
	}

	// Check http_panics_total
	httpPanics, ok := panicRecovery["http_panics_total"].(float64)
	if !ok {
		t.Fatal("Expected http_panics_total to be a number")
	}

	if httpPanics < 1 {
		t.Errorf("Expected at least 1 panic, got %f", httpPanics)
	}

	// Check last_http_panic is set
	lastPanic, ok := panicRecovery["last_http_panic"].(string)
	if !ok {
		t.Fatal("Expected last_http_panic to be a string")
	}

	if lastPanic == "never" {
		t.Error("Expected last_http_panic to be set to a timestamp")
	}

	// Check tokenizer metrics exist
	tokenizerPanics, ok := panicRecovery["tokenizer_panics_total"]
	if !ok {
		t.Error("Expected tokenizer_panics_total in metrics")
	}

	tokenizerFallback, ok := panicRecovery["tokenizer_fallback_used"]
	if !ok {
		t.Error("Expected tokenizer_fallback_used in metrics")
	}

	// These should be numbers (even if zero)
	if _, ok := tokenizerPanics.(float64); !ok {
		t.Errorf("Expected tokenizer_panics_total to be a number, got %T", tokenizerPanics)
	}

	if _, ok := tokenizerFallback.(float64); !ok {
		t.Errorf("Expected tokenizer_fallback_used to be a number, got %T", tokenizerFallback)
	}
}

// TestPanicRecoveryLogging tests that panic details are logged
func TestPanicRecoveryLogging(t *testing.T) {
	cfg := &config.Config{CacheStrategy: "moderate"}
	logger := logrus.New()

	// Capture logs
	var logOutput bytes.Buffer
	logger.SetOutput(&logOutput)
	logger.SetLevel(logrus.ErrorLevel)

	handler := NewAutocacheHandler(cfg, logger)

	// Create a handler that panics
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic with context")
	})

	wrapped := handler.PanicRecoveryMiddleware(panicHandler)

	// Create test request with headers
	req := httptest.NewRequest("GET", "/test/path", nil)
	req.Header.Set("User-Agent", "TestAgent/1.0")
	req.RemoteAddr = "192.168.1.100:12345"

	rr := httptest.NewRecorder()
	wrapped.ServeHTTP(rr, req)

	// Check log output contains panic details
	logStr := logOutput.String()

	expectedStrings := []string{
		"test panic with context", // panic value
		"/test/path",               // URL
		"GET",                      // method
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(logStr, expected) {
			t.Errorf("Expected log to contain %q, log output: %s", expected, logStr)
		}
	}
}