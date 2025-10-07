package tokenizer

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"autocache/internal/types"
)

// Tokenizer interface for counting tokens
type Tokenizer interface {
	CountTokens(text string) int
	CountMessageTokens(message types.Message) int
	CountContentBlockTokens(block types.ContentBlock) int
	CountToolTokens(tool types.ToolDefinition) int
	GetModelMinimumTokens(model string) int
	CountSystemTokens(system string) int
	CountSystemBlocksTokens(blocks []types.ContentBlock) int
	EstimateRequestTokens(req *types.AnthropicRequest) int
	GetTokenCountForCaching(text string, model string, strategy types.CacheStrategy) (int, int)
}

// AnthropicTokenizer implements the Tokenizer interface
// This is a simplified approximation based on Claude's tokenization
type AnthropicTokenizer struct {
	// Cache for repeated calculations
	cache map[string]int
	mu    sync.RWMutex // Mutex to protect concurrent access to cache
}

// NewAnthropicTokenizer creates a new tokenizer instance
func NewAnthropicTokenizer() *AnthropicTokenizer {
	return &AnthropicTokenizer{
		cache: make(map[string]int),
	}
}

// CountTokens estimates token count for text
// This is a simplified approximation - ideally we'd use the official tokenizer
func (t *AnthropicTokenizer) CountTokens(text string) int {
	if text == "" {
		return 0
	}

	// Check cache first (read lock)
	t.mu.RLock()
	count, exists := t.cache[text]
	t.mu.RUnlock()

	if exists {
		return count
	}

	// Rough approximation based on Claude's tokenization patterns
	// This is conservative and may overestimate slightly

	// Count characters and apply heuristics
	charCount := utf8.RuneCountInString(text)

	// Based on real API data: "You are helpful" (15 chars) = 8 tokens = 1.9 chars/token
	// But this varies widely. Let's overestimate significantly to increase chances of hitting cache threshold
	// We'll use 1.5 chars/token to significantly overestimate
	baseTokens := float64(charCount) / 1.5

	// Adjust for different content types
	tokens := baseTokens

	// Code tends to have more tokens due to syntax
	if isCodeLike(text) {
		tokens *= 1.2
	}

	// JSON/structured data is more token-dense
	if isJSONLike(text) {
		tokens *= 1.3
	}

	// Very short text tends to have higher token density
	if charCount < 50 {
		tokens *= 1.1
	}

	// Add overhead for message formatting
	if charCount > 1000 {
		tokens += 2 // Message structure overhead
	}

	result := int(tokens)
	if result < 1 && charCount > 0 {
		result = 1 // Minimum 1 token for any content
	}

	// Cache the result (write lock)
	t.mu.Lock()
	t.cache[text] = result
	t.mu.Unlock()

	return result
}

// CountMessageTokens counts tokens in a complete message
func (t *AnthropicTokenizer) CountMessageTokens(message types.Message) int {
	total := 0

	// Add overhead for message structure
	total += 3 // Role and message wrapping

	// Count content blocks
	for _, block := range message.Content {
		total += t.CountContentBlockTokens(block)
	}

	return total
}

// CountContentBlockTokens counts tokens in a content block
func (t *AnthropicTokenizer) CountContentBlockTokens(block types.ContentBlock) int {
	total := 0

	// Add overhead for content block structure
	total += 2

	switch block.Type {
	case "text":
		total += t.CountTokens(block.Text)
	case "image":
		// Images have a fixed token cost based on size/resolution
		// For now, we'll use a conservative estimate
		total += 85 // Base cost for image processing
	}

	return total
}

// CountToolTokens counts tokens in a tool definition
func (t *AnthropicTokenizer) CountToolTokens(tool types.ToolDefinition) int {
	total := 0

	// Tool structure overhead
	total += 5

	// Name and description
	total += t.CountTokens(tool.Name)
	total += t.CountTokens(tool.Description)

	// Input schema (serialize and count)
	if tool.InputSchema != nil {
		// Rough estimate for JSON schema - this could be more precise
		schemaStr := fmt.Sprintf("%v", tool.InputSchema)
		total += t.CountTokens(schemaStr)
	}

	return total
}

// GetModelMinimumTokens returns the minimum tokens required for caching by model
func (t *AnthropicTokenizer) GetModelMinimumTokens(model string) int {
	// Model-specific minimums based on Anthropic documentation
	haikuModels := []string{
		"claude-3-haiku-20240307",
		"claude-3-5-haiku-20241022",
	}

	// Check if it's a Haiku model
	for _, haikuModel := range haikuModels {
		if strings.Contains(model, "haiku") || model == haikuModel {
			return 2048
		}
	}

	// Default for most models (Sonnet, Opus, etc.)
	return 1024
}

// CountSystemTokens counts tokens in system prompt
func (t *AnthropicTokenizer) CountSystemTokens(system string) int {
	if system == "" {
		return 0
	}

	// System prompt has some overhead
	return t.CountTokens(system) + 2
}

// CountSystemBlocksTokens counts tokens in system blocks
func (t *AnthropicTokenizer) CountSystemBlocksTokens(blocks []types.ContentBlock) int {
	total := 2 // System structure overhead

	for _, block := range blocks {
		total += t.CountContentBlockTokens(block)
	}

	return total
}

// EstimateRequestTokens provides a complete token estimate for a request
func (t *AnthropicTokenizer) EstimateRequestTokens(req *types.AnthropicRequest) int {
	total := 0

	// Base request overhead
	total += 5

	// System content
	if req.System != "" {
		total += t.CountSystemTokens(req.System)
	}

	if len(req.SystemBlocks) > 0 {
		total += t.CountSystemBlocksTokens(req.SystemBlocks)
	}

	// Tools
	for _, tool := range req.Tools {
		total += t.CountToolTokens(tool)
	}

	// Messages
	for _, message := range req.Messages {
		total += t.CountMessageTokens(message)
	}

	return total
}

// Helper functions

func isCodeLike(text string) bool {
	// Simple heuristics for code detection
	codePatterns := []string{
		`\{.*\}`,     // Curly braces
		`\[.*\]`,     // Square brackets
		`function`,   // Function keyword
		`class`,      // Class keyword
		`import`,     // Import statements
		`def `,       // Python functions
		`var `,       // Variable declarations
		`const `,     // Constants
		`let `,       // Let declarations
		`if\s*\(`,    // If statements
		`for\s*\(`,   // For loops
		`while\s*\(`, // While loops
	}

	for _, pattern := range codePatterns {
		if matched, _ := regexp.MatchString(pattern, text); matched {
			return true
		}
	}

	// Check for high punctuation density
	punctCount := 0
	for _, char := range text {
		if strings.ContainsRune("{}[]().,;:\"'`<>=+-*/&|^%!", char) {
			punctCount++
		}
	}

	// If more than 15% punctuation, likely code
	if len(text) > 20 && float64(punctCount)/float64(len(text)) > 0.15 {
		return true
	}

	return false
}

func isJSONLike(text string) bool {
	trimmed := strings.TrimSpace(text)
	return (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		   (strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))
}

// GetTokenCountForCaching returns the token count with strategy multiplier applied
func (t *AnthropicTokenizer) GetTokenCountForCaching(text string, model string, strategy types.CacheStrategy) (int, int) {
	tokens := t.CountTokens(text)
	minimumRequired := t.GetModelMinimumTokens(model)

	// Apply strategy multiplier
	config := types.GetStrategyConfig(strategy)
	adjustedMinimum := int(float64(minimumRequired) * config.MinTokensMultiplier)

	return tokens, adjustedMinimum
}