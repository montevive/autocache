package tokenizer

import (
	"encoding/json"
	"fmt"
	"strings"

	"autocache/internal/types"

	tokenizer "github.com/qhenkart/anthropic-tokenizer-go"
	"github.com/sirupsen/logrus"
)

// AnthropicRealTokenizer implements the Tokenizer interface using the official Anthropic tokenizer
// This provides accurate token counting that matches Anthropic's actual API
type AnthropicRealTokenizer struct {
	tokenizer *tokenizer.Tokenizer
	logger    *logrus.Logger
}

// NewAnthropicRealTokenizer creates a new tokenizer instance using the official Anthropic tokenizer
// WARNING: This is expensive to initialize! Create once and reuse.
func NewAnthropicRealTokenizer() (*AnthropicRealTokenizer, error) {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)
	return NewAnthropicRealTokenizerWithLogger(logger)
}

// NewAnthropicRealTokenizerWithLogger creates a new tokenizer with a custom logger
func NewAnthropicRealTokenizerWithLogger(logger *logrus.Logger) (*AnthropicRealTokenizer, error) {
	// Initialize tokenizer (expensive operation)
	tk, err := tokenizer.New()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize anthropic tokenizer: %w", err)
	}
	if tk == nil {
		return nil, fmt.Errorf("failed to initialize anthropic tokenizer: nil tokenizer")
	}

	logger.Info("Anthropic real tokenizer initialized successfully")

	return &AnthropicRealTokenizer{
		tokenizer: tk,
		logger:    logger,
	}, nil
}

// CountTokens counts tokens in a text string
func (art *AnthropicRealTokenizer) CountTokens(text string) int {
	if text == "" {
		return 0
	}

	// Use the official anthropic tokenizer
	count := art.tokenizer.Tokens(text)
	return count
}

// CountMessageTokens counts tokens in a complete message
func (art *AnthropicRealTokenizer) CountMessageTokens(message types.Message) int {
	total := 0

	// Add overhead for message structure (role, wrapping)
	// Based on Anthropic's message format
	total += 3

	// Count content blocks
	for _, block := range message.Content {
		total += art.CountContentBlockTokens(block)
	}

	return total
}

// CountContentBlockTokens counts tokens in a content block
func (art *AnthropicRealTokenizer) CountContentBlockTokens(block types.ContentBlock) int {
	total := 0

	// Add overhead for content block structure
	total += 2

	switch block.Type {
	case "text":
		total += art.CountTokens(block.Text)
	case "image":
		// Images have a fixed token cost
		// Anthropic's image token calculation is complex, using conservative estimate
		total += 85
	}

	return total
}

// CountToolTokens counts tokens in a tool definition
func (art *AnthropicRealTokenizer) CountToolTokens(tool types.ToolDefinition) int {
	total := 0

	// Tool structure overhead
	total += 5

	// Name and description
	total += art.CountTokens(tool.Name)
	total += art.CountTokens(tool.Description)

	// Input schema (serialize to JSON and count)
	if tool.InputSchema != nil {
		schemaJSON, err := json.Marshal(tool.InputSchema)
		if err != nil {
			// Fallback to string representation if marshal fails
			schemaStr := fmt.Sprintf("%v", tool.InputSchema)
			total += art.CountTokens(schemaStr)
		} else {
			total += art.CountTokens(string(schemaJSON))
		}
	}

	return total
}

// GetModelMinimumTokens returns the minimum tokens required for caching by model
func (art *AnthropicRealTokenizer) GetModelMinimumTokens(model string) int {
	// Model-specific minimums based on Anthropic documentation
	// https://docs.anthropic.com/en/docs/build-with-claude/prompt-caching

	// Haiku models require 2048 tokens
	haikuModels := []string{
		"claude-3-haiku-20240307",
		"claude-3-5-haiku-20241022",
	}

	for _, haikuModel := range haikuModels {
		if strings.Contains(model, "haiku") || model == haikuModel {
			return 2048
		}
	}

	// Default for Sonnet, Opus, and other models
	return 1024
}

// CountSystemTokens counts tokens in system prompt
func (art *AnthropicRealTokenizer) CountSystemTokens(system string) int {
	if system == "" {
		return 0
	}

	// System prompt has overhead for structure
	return art.CountTokens(system) + 2
}

// CountSystemBlocksTokens counts tokens in system blocks
func (art *AnthropicRealTokenizer) CountSystemBlocksTokens(blocks []types.ContentBlock) int {
	total := 2 // System structure overhead

	for _, block := range blocks {
		total += art.CountContentBlockTokens(block)
	}

	return total
}

// EstimateRequestTokens provides a complete token estimate for a request
func (art *AnthropicRealTokenizer) EstimateRequestTokens(req *types.AnthropicRequest) int {
	total := 0

	// Base request overhead
	total += 5

	// System content
	if req.System != "" {
		total += art.CountSystemTokens(req.System)
	}

	if len(req.SystemBlocks) > 0 {
		total += art.CountSystemBlocksTokens(req.SystemBlocks)
	}

	// Tools
	for _, tool := range req.Tools {
		total += art.CountToolTokens(tool)
	}

	// Messages
	for _, message := range req.Messages {
		total += art.CountMessageTokens(message)
	}

	return total
}

// GetTokenCountForCaching returns the token count with strategy multiplier applied
func (art *AnthropicRealTokenizer) GetTokenCountForCaching(text string, model string, strategy types.CacheStrategy) (int, int) {
	tokens := art.CountTokens(text)
	minimumRequired := art.GetModelMinimumTokens(model)

	// Apply strategy multiplier
	config := types.GetStrategyConfig(strategy)
	adjustedMinimum := int(float64(minimumRequired) * config.MinTokensMultiplier)

	return tokens, adjustedMinimum
}
