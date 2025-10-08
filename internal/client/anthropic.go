package client

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"autocache/internal/types"

	"github.com/sirupsen/logrus"
)

// ProxyClient handles communication with the Anthropic API
type ProxyClient struct {
	httpClient  *http.Client
	anthropicURL string
	logger      *logrus.Logger
}

// NewProxyClient creates a new proxy client
func NewProxyClient(anthropicURL string, logger *logrus.Logger) *ProxyClient {
	return &ProxyClient{
		httpClient: &http.Client{
			Timeout: 300 * time.Second, // 5 minute timeout for long requests
		},
		anthropicURL: anthropicURL,
		logger:       logger,
	}
}

// ForwardRequest forwards a request to the Anthropic API
func (pc *ProxyClient) ForwardRequest(req *types.AnthropicRequest, headers map[string]string) (*http.Response, error) {
	// Serialize the request
	requestBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	pc.logger.WithFields(logrus.Fields{
		"model":     req.Model,
		"url":       pc.anthropicURL + "/v1/messages",
		"body_size": len(requestBody),
	}).Debug("Forwarding request to Anthropic API")

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", pc.anthropicURL+"/v1/messages", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01") // Required by Anthropic API

	// Forward original headers (especially Authorization)
	for key, value := range headers {
		// Skip headers that might interfere
		if !shouldSkipHeader(key) {
			httpReq.Header.Set(key, value)
			if strings.ToLower(key) == "x-api-key" {
				pc.logger.WithFields(logrus.Fields{
					"header_key":      key,
					"api_key_preview": maskAPIKey(value),
				}).Debug("Setting x-api-key header for Anthropic request")
			}
		}
	}

	// Debug: Check if x-api-key was actually set
	actualAPIKey := httpReq.Header.Get("x-api-key")
	pc.logger.WithFields(logrus.Fields{
		"x-api-key_present": actualAPIKey != "",
		"api_key_preview":   maskAPIKey(actualAPIKey),
	}).Debug("Final headers before sending to Anthropic")

	// Make the request
	resp, err := pc.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to Anthropic API: %w", err)
	}

	pc.logger.WithFields(logrus.Fields{
		"status_code":    resp.StatusCode,
		"content_length": resp.ContentLength,
	}).Debug("Received response from Anthropic API")

	return resp, nil
}

// ForwardStreamingRequest forwards a streaming request to the Anthropic API
func (pc *ProxyClient) ForwardStreamingRequest(req *types.AnthropicRequest, headers map[string]string, responseWriter http.ResponseWriter) error {
	// Serialize the request
	requestBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	pc.logger.WithFields(logrus.Fields{
		"model":     req.Model,
		"streaming": true,
		"url":       pc.anthropicURL + "/v1/messages",
	}).Debug("Forwarding streaming request to Anthropic API")

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", pc.anthropicURL+"/v1/messages", bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("anthropic-version", "2023-06-01") // Required by Anthropic API

	// Forward original headers
	for key, value := range headers {
		if !shouldSkipHeader(key) {
			httpReq.Header.Set(key, value)
			if strings.ToLower(key) == "x-api-key" {
				pc.logger.WithFields(logrus.Fields{
					"header_key":      key,
					"api_key_preview": maskAPIKey(value),
				}).Debug("Setting x-api-key header for Anthropic streaming request")
			}
		}
	}

	// Debug: Check if x-api-key was actually set
	actualAPIKey := httpReq.Header.Get("x-api-key")
	pc.logger.WithFields(logrus.Fields{
		"x-api-key_present": actualAPIKey != "",
		"api_key_preview":   maskAPIKey(actualAPIKey),
	}).Debug("Final headers before sending streaming request to Anthropic")

	// Make the request
	resp, err := pc.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to make request to Anthropic API: %w", err)
	}
	defer resp.Body.Close()

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			responseWriter.Header().Add(key, value)
		}
	}

	// Set status code
	responseWriter.WriteHeader(resp.StatusCode)

	// Stream the response
	_, err = io.Copy(responseWriter, resp.Body)
	if err != nil {
		pc.logger.WithError(err).Error("Failed to stream response")
		return fmt.Errorf("failed to stream response: %w", err)
	}

	return nil
}

// ReadAndParseResponse reads and parses a non-streaming response
func (pc *ProxyClient) ReadAndParseResponse(resp *http.Response) (*types.AnthropicResponse, []byte, error) {
	defer resp.Body.Close()

	// Handle gzip compression if present
	reader := resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader

		pc.logger.Debug("Response is gzip-compressed, decompressing")
	}

	// Read the (possibly decompressed) response body
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		pc.logger.WithFields(logrus.Fields{
			"status_code": resp.StatusCode,
			"body":        string(body),
		}).Error("Anthropic API returned error")

		return nil, body, fmt.Errorf("Anthropic API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var anthropicResp types.AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		pc.logger.WithFields(logrus.Fields{
			"body": string(body),
		}).Error("Failed to parse Anthropic response")

		return nil, body, fmt.Errorf("failed to parse response: %w", err)
	}

	pc.logger.WithFields(logrus.Fields{
		"id":            anthropicResp.ID,
		"model":         anthropicResp.Model,
		"input_tokens":  anthropicResp.Usage.InputTokens,
		"output_tokens": anthropicResp.Usage.OutputTokens,
		"cache_creation": anthropicResp.Usage.CacheCreationInputTokens,
		"cache_read":    anthropicResp.Usage.CacheReadInputTokens,
	}).Info("Successfully parsed Anthropic response")

	return &anthropicResp, body, nil
}

// ValidateRequest performs basic validation on the request
func (pc *ProxyClient) ValidateRequest(req *types.AnthropicRequest) error {
	if req.Model == "" {
		return fmt.Errorf("model is required")
	}

	if req.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive")
	}

	if len(req.Messages) == 0 {
		return fmt.Errorf("at least one message is required")
	}

	// Validate message structure
	for i, message := range req.Messages {
		if message.Role == "" {
			return fmt.Errorf("message %d is missing role", i)
		}

		if len(message.Content) == 0 {
			return fmt.Errorf("message %d has no content", i)
		}

		// Check for valid roles
		validRoles := map[string]bool{
			"user":      true,
			"assistant": true,
		}

		if !validRoles[message.Role] {
			return fmt.Errorf("message %d has invalid role: %s", i, message.Role)
		}
	}

	return nil
}

// ExtractAPIKey extracts the API key from request headers
func ExtractAPIKey(headers http.Header, logger *logrus.Logger) string {
	// Log all headers for debugging
	headerList := make([]string, 0)
	for key := range headers {
		headerList = append(headerList, key)
	}
	logger.WithFields(logrus.Fields{
		"headers_present": headerList,
	}).Debug("Extracting API key from headers")

	// Check Authorization header
	auth := headers.Get("Authorization")
	if auth != "" {
		logger.Debug("Found API key in Authorization header")
		// Remove "Bearer " prefix if present
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
		return auth
	}

	// Check x-api-key header
	apiKey := headers.Get("x-api-key")
	if apiKey != "" {
		logger.WithFields(logrus.Fields{
			"api_key_preview": maskAPIKey(apiKey),
		}).Debug("Found API key in x-api-key header")
		return apiKey
	}

	// Check anthropic-api-key header
	anthropicKey := headers.Get("anthropic-api-key")
	if anthropicKey != "" {
		logger.Debug("Found API key in anthropic-api-key header")
		return anthropicKey
	}

	logger.Debug("No API key found in request headers")
	return ""
}

// SetupAuthHeader sets up the authorization header for the Anthropic API
func SetupAuthHeader(headers map[string]string, apiKey string, logger *logrus.Logger) {
	if apiKey != "" {
		logger.WithFields(logrus.Fields{
			"api_key_preview": maskAPIKey(apiKey),
		}).Debug("Setting up x-api-key header for Anthropic API")

		// Remove all possible variations of API key headers (case-insensitive)
		headersToRemove := []string{}
		for key := range headers {
			lowerKey := strings.ToLower(key)
			if lowerKey == "authorization" || lowerKey == "x-api-key" || lowerKey == "anthropic-api-key" {
				headersToRemove = append(headersToRemove, key)
			}
		}
		for _, key := range headersToRemove {
			delete(headers, key)
			logger.WithField("removed_header", key).Debug("Removed existing auth header")
		}

		// Anthropic expects the API key in the x-api-key header
		headers["x-api-key"] = apiKey
	}
}

// maskAPIKey masks the API key for logging (shows first 10 chars)
func maskAPIKey(apiKey string) string {
	if apiKey == "" {
		return "<empty>"
	}
	if len(apiKey) <= 10 {
		return "***"
	}
	return apiKey[:10] + "***"
}

// shouldSkipHeader determines if a header should be skipped when forwarding
func shouldSkipHeader(header string) bool {
	skipHeaders := map[string]bool{
		"content-length":    true,
		"transfer-encoding": true,
		"connection":        true,
		"upgrade":           true,
		"proxy-connection":  true,
		"proxy-authorization": true,
		"te":                true,
		"trailer":           true,
		"host":              true,
	}

	return skipHeaders[strings.ToLower(header)]
}

// CreateHeadersMap creates a map of headers to forward
func CreateHeadersMap(reqHeaders http.Header, apiKey string, logger *logrus.Logger) map[string]string {
	headers := make(map[string]string)

	// Copy relevant headers
	for key, values := range reqHeaders {
		if !shouldSkipHeader(key) && len(values) > 0 {
			headers[key] = values[0] // Take first value
		}
	}

	logger.WithFields(logrus.Fields{
		"headers_before_auth": len(headers),
		"has_api_key":         apiKey != "",
		"api_key_preview":     maskAPIKey(apiKey),
	}).Debug("Headers before authentication setup")

	// Setup authentication
	if apiKey != "" {
		SetupAuthHeader(headers, apiKey, logger)
	}

	logger.WithFields(logrus.Fields{
		"headers_after_auth": len(headers),
		"x-api-key_set":      headers["x-api-key"] != "",
	}).Debug("Headers after authentication setup")

	return headers
}

// IsStreamingRequest checks if the request is for streaming
func IsStreamingRequest(req *types.AnthropicRequest) bool {
	return req.Stream != nil && *req.Stream
}

// LogRequestSummary logs a summary of the request for debugging
func (pc *ProxyClient) LogRequestSummary(req *types.AnthropicRequest) {
	pc.logger.WithFields(logrus.Fields{
		"model":         req.Model,
		"max_tokens":    req.MaxTokens,
		"messages":      len(req.Messages),
		"system_length": len(req.System),
		"tools":         len(req.Tools),
		"temperature":   req.Temperature,
		"streaming":     IsStreamingRequest(req),
	}).Info("Processing request")
}