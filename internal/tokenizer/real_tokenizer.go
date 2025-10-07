package tokenizer

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"autocache/internal/types"

	"github.com/sirupsen/logrus"
)

// RealTokenizer uses Anthropic's official token counting API
type RealTokenizer struct {
	httpClient    *http.Client
	anthropicURL  string
	apiKey        string
	logger        *logrus.Logger
	cache         map[string]int // Cache for repeated calculations
}

// TokenCountRequest represents the request to the token counting API
type TokenCountRequest struct {
	Model        string                 `json:"model"`
	System       string                 `json:"system,omitempty"`
	SystemBlocks []types.ContentBlock   `json:"-"` // Handle separately to avoid JSON conflict
	Messages     []types.Message        `json:"messages,omitempty"`
	Tools        []types.ToolDefinition `json:"tools,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TokenCountResponse represents the response from the token counting API
type TokenCountResponse struct {
	InputTokens int `json:"input_tokens"`
}

// NewRealTokenizer creates a new real tokenizer instance
func NewRealTokenizer(anthropicURL, apiKey string, logger *logrus.Logger) *RealTokenizer {
	return &RealTokenizer{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		anthropicURL: anthropicURL,
		apiKey:       apiKey,
		logger:       logger,
		cache:        make(map[string]int),
	}
}

// CountTokens estimates token count for text using the real API
func (rt *RealTokenizer) CountTokens(text string) int {
	if text == "" {
		return 0
	}

	// Check cache first
	if count, exists := rt.cache[text]; exists {
		return count
	}

	// Create a minimal request to count just this text
	req := TokenCountRequest{
		Model: "claude-3-5-sonnet-20241022", // Default model for counting
		Messages: []types.Message{{
			Role: "user",
			Content: []types.ContentBlock{{
				Type: "text",
				Text: text,
			}},
		}},
	}

	count := rt.countTokensViaAPI(req)

	// Cache the result
	rt.cache[text] = count
	return count
}

// CountSystemTokens counts tokens in system prompt using real API
func (rt *RealTokenizer) CountSystemTokens(system string) int {
	if system == "" {
		return 0
	}

	// Create request with only system prompt
	req := TokenCountRequest{
		Model:  "claude-3-5-sonnet-20241022",
		System: system,
		Messages: []types.Message{{
			Role: "user",
			Content: []types.ContentBlock{{
				Type: "text",
				Text: "Hi", // Minimal user message required
			}},
		}},
	}

	// Get total count, then subtract the minimal user message (approximately 3-5 tokens)
	totalCount := rt.countTokensViaAPI(req)

	// Subtract estimated tokens for minimal user message
	systemTokens := totalCount - 5 // Conservative estimate for "Hi" message structure
	if systemTokens < 0 {
		systemTokens = 0
	}

	return systemTokens
}

// CountRequestTokens provides a complete token count for a request using real API
func (rt *RealTokenizer) CountRequestTokens(req *types.AnthropicRequest) int {
	// Convert to token count request
	tokenReq := TokenCountRequest{
		Model:        req.Model,
		System:       req.System,
		SystemBlocks: req.SystemBlocks,
		Messages:     req.Messages,
		Tools:        req.Tools,
	}

	return rt.countTokensViaAPI(tokenReq)
}

// countTokensViaAPI makes the actual API call to count tokens
func (rt *RealTokenizer) countTokensViaAPI(req TokenCountRequest) int {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Serialize the request
	requestBody, err := json.Marshal(req)
	if err != nil {
		rt.logger.WithError(err).Error("Failed to marshal token count request")
		return rt.fallbackTokenCount(req) // Fallback to estimation
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", rt.anthropicURL+"/v1/messages/count_tokens", bytes.NewBuffer(requestBody))
	if err != nil {
		rt.logger.WithError(err).Error("Failed to create token count HTTP request")
		return rt.fallbackTokenCount(req)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", rt.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Make the request
	resp, err := rt.httpClient.Do(httpReq)
	if err != nil {
		rt.logger.WithError(err).Error("Failed to make token count request")
		return rt.fallbackTokenCount(req)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		rt.logger.WithError(err).Error("Failed to read token count response")
		return rt.fallbackTokenCount(req)
	}

	// Check for non-200 status
	if resp.StatusCode != http.StatusOK {
		rt.logger.WithFields(logrus.Fields{
			"status_code": resp.StatusCode,
			"body":        string(body),
		}).Error("Token count API returned error")
		return rt.fallbackTokenCount(req)
	}

	// Parse response
	var tokenResp TokenCountResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		rt.logger.WithError(err).Error("Failed to parse token count response")
		return rt.fallbackTokenCount(req)
	}

	rt.logger.WithFields(logrus.Fields{
		"input_tokens": tokenResp.InputTokens,
		"model":        req.Model,
	}).Debug("Successfully counted tokens via API")

	return tokenResp.InputTokens
}

// fallbackTokenCount provides a fallback estimation when API is unavailable
func (rt *RealTokenizer) fallbackTokenCount(req TokenCountRequest) int {
	rt.logger.Debug("Using fallback token counting")

	// Simple character-based estimation as fallback
	total := 0

	// Count system tokens
	if req.System != "" {
		total += len(req.System) / 4 // Conservative 4 chars per token
	}

	// Count message tokens
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			if block.Type == "text" {
				total += len(block.Text) / 4
			}
		}
		total += 5 // Message structure overhead
	}

	// Count tool tokens
	for _, tool := range req.Tools {
		total += len(tool.Name) / 4
		total += len(tool.Description) / 4
		total += 20 // Tool structure overhead
	}

	return total
}

// Implement remaining Tokenizer interface methods by delegating to the real API

func (rt *RealTokenizer) CountMessageTokens(message types.Message) int {
	req := TokenCountRequest{
		Model:    "claude-3-5-sonnet-20241022",
		Messages: []types.Message{message},
	}
	return rt.countTokensViaAPI(req)
}

func (rt *RealTokenizer) CountContentBlockTokens(block types.ContentBlock) int {
	if block.Type == "text" {
		return rt.CountTokens(block.Text)
	}
	// For images, use a conservative estimate
	return 85
}

func (rt *RealTokenizer) CountToolTokens(tool types.ToolDefinition) int {
	req := TokenCountRequest{
		Model: "claude-3-5-sonnet-20241022",
		Tools: []types.ToolDefinition{tool},
		Messages: []types.Message{{
			Role: "user",
			Content: []types.ContentBlock{{
				Type: "text",
				Text: "Hi",
			}},
		}},
	}

	totalCount := rt.countTokensViaAPI(req)
	// Subtract minimal user message
	toolTokens := totalCount - 5
	if toolTokens < 0 {
		toolTokens = 10 // Minimum reasonable tool token count
	}
	return toolTokens
}

func (rt *RealTokenizer) GetModelMinimumTokens(model string) int {
	// Model-specific minimums based on Anthropic documentation
	if model == "claude-3-haiku-20240307" || model == "claude-3-5-haiku-20241022" {
		return 2048
	}
	return 1024
}

func (rt *RealTokenizer) CountSystemBlocksTokens(blocks []types.ContentBlock) int {
	req := TokenCountRequest{
		Model:        "claude-3-5-sonnet-20241022",
		SystemBlocks: blocks,
		Messages: []types.Message{{
			Role: "user",
			Content: []types.ContentBlock{{
				Type: "text",
				Text: "Hi",
			}},
		}},
	}

	totalCount := rt.countTokensViaAPI(req)
	// Subtract minimal user message
	systemTokens := totalCount - 5
	if systemTokens < 0 {
		systemTokens = 0
	}
	return systemTokens
}

// EstimateRequestTokens provides a complete token estimate for a request
func (rt *RealTokenizer) EstimateRequestTokens(req *types.AnthropicRequest) int {
	return rt.CountRequestTokens(req)
}

// GetTokenCountForCaching returns the token count with strategy multiplier applied
func (rt *RealTokenizer) GetTokenCountForCaching(text string, model string, strategy types.CacheStrategy) (int, int) {
	tokens := rt.CountTokens(text)
	minimumRequired := rt.GetModelMinimumTokens(model)

	// Apply strategy multiplier
	config := types.GetStrategyConfig(strategy)
	adjustedMinimum := int(float64(minimumRequired) * config.MinTokensMultiplier)

	return tokens, adjustedMinimum
}