package tokenizer

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"autocache/internal/types"

	"github.com/sirupsen/logrus"
	"github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/pretrained"
)

//go:embed claude-v3-tokenizer.json
var tokenizerJSON []byte

// OfflineTokenizer implements Claude tokenization using embedded tokenizer data
type OfflineTokenizer struct {
	tokenizer        *tokenizer.Tokenizer
	fallbackTokenizer *AnthropicTokenizer
	logger           *logrus.Logger
	mu               sync.RWMutex
	panicCount       atomic.Uint64
	fallbackCount    atomic.Uint64
}

// NewOfflineTokenizer creates a new offline tokenizer instance
func NewOfflineTokenizer() (*OfflineTokenizer, error) {
	return NewOfflineTokenizerWithLogger(nil)
}

// NewOfflineTokenizerWithLogger creates a new offline tokenizer instance with a logger
func NewOfflineTokenizerWithLogger(logger *logrus.Logger) (*OfflineTokenizer, error) {
	// Write the embedded JSON to a temporary file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "claude-v3-tokenizer.json")

	err := os.WriteFile(tmpFile, tokenizerJSON, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write tokenizer to temp file: %w", err)
	}

	// Load tokenizer from the temp file
	tk, err := pretrained.FromFile(tmpFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load tokenizer: %w", err)
	}

	// Use a no-op logger if none provided
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.WarnLevel)
	}

	return &OfflineTokenizer{
		tokenizer:         tk,
		fallbackTokenizer: NewAnthropicTokenizer(),
		logger:            logger,
	}, nil
}

// CountTokens counts tokens in text using the offline tokenizer
// Falls back to heuristic tokenizer on panic or error
func (ot *OfflineTokenizer) CountTokens(text string) int {
	if text == "" {
		return 0
	}

	// Use panic recovery to handle tokenizer library bugs
	var result int
	var panicOccurred bool
	var panicValue interface{}
	var panicStack []byte

	func() {
		defer func() {
			if r := recover(); r != nil {
				panicOccurred = true
				panicValue = r
				panicStack = debug.Stack()
			}
		}()

		ot.mu.RLock()
		defer ot.mu.RUnlock()

		// Encode the text
		encoding, err := ot.tokenizer.EncodeSingle(text)
		if err != nil {
			// On error, we'll use fallback outside this function
			return
		}

		result = len(encoding.Ids)
	}()

	// Handle panic or error by using fallback tokenizer
	if panicOccurred || result == 0 {
		if panicOccurred {
			ot.panicCount.Add(1)
			ot.logTokenizerPanic(text, panicValue, panicStack)
		}

		ot.fallbackCount.Add(1)
		result = ot.fallbackTokenizer.CountTokens(text)

		ot.logger.WithFields(logrus.Fields{
			"text_length":     len(text),
			"fallback_tokens": result,
			"panic_occurred":  panicOccurred,
		}).Debug("Using fallback tokenizer")
	}

	return result
}

// logTokenizerPanic logs detailed information about a tokenizer panic
func (ot *OfflineTokenizer) logTokenizerPanic(text string, panicValue interface{}, stack []byte) {
	// Truncate text sample for logging (prevent log spam)
	const maxSampleLen = 200
	textSample := text
	if len(text) > maxSampleLen {
		textSample = text[:maxSampleLen] + "... (truncated)"
	}

	// Sanitize text sample (remove newlines, control chars)
	textSample = sanitizeForLog(textSample)

	// Determine text characteristics
	textChars := map[string]interface{}{
		"length":      len(text),
		"is_code":     isCodeLike(text),
		"is_json":     isJSONLike(text),
		"has_unicode": hasNonASCII(text),
	}

	ot.logger.WithFields(logrus.Fields{
		"panic_value":       fmt.Sprintf("%v", panicValue),
		"text_sample":       textSample,
		"text_stats":        textChars,
		"total_panic_count": ot.panicCount.Load(),
		"stack_trace":       string(stack),
	}).Error("Tokenizer panic recovered - falling back to heuristic tokenizer")
}

// GetPanicStats returns statistics about tokenizer panics
func (ot *OfflineTokenizer) GetPanicStats() map[string]uint64 {
	return map[string]uint64{
		"panic_count":    ot.panicCount.Load(),
		"fallback_count": ot.fallbackCount.Load(),
	}
}

// CountSystemTokens counts tokens in system prompt
func (ot *OfflineTokenizer) CountSystemTokens(system string) int {
	if system == "" {
		return 0
	}

	// System prompts are counted the same way as regular text
	return ot.CountTokens(system)
}

// CountSystemBlocksTokens counts tokens in system blocks
func (ot *OfflineTokenizer) CountSystemBlocksTokens(blocks []types.ContentBlock) int {
	total := 0
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			total += ot.CountTokens(block.Text)
		}
	}
	// Add overhead for block structure
	total += len(blocks) * 3
	return total
}

// CountMessageTokens counts tokens in a message
func (ot *OfflineTokenizer) CountMessageTokens(message types.Message) int {
	total := 0

	// Count role overhead
	total += 3

	// Count content blocks
	for _, block := range message.Content {
		total += ot.CountContentBlockTokens(block)
	}

	return total
}

// CountContentBlockTokens counts tokens in a content block
func (ot *OfflineTokenizer) CountContentBlockTokens(block types.ContentBlock) int {
	switch block.Type {
	case "text":
		if block.Text != "" {
			return ot.CountTokens(block.Text) + 2 // Add structure overhead
		}
	case "image":
		// Image tokens are approximately 85 per image (from Claude docs)
		return 85
	}
	return 0
}

// CountToolTokens counts tokens in a tool definition
func (ot *OfflineTokenizer) CountToolTokens(tool types.ToolDefinition) int {
	total := 0

	// Count name
	if tool.Name != "" {
		total += ot.CountTokens(tool.Name)
	}

	// Count description
	if tool.Description != "" {
		total += ot.CountTokens(tool.Description)
	}

	// Count input schema (convert to JSON and count)
	if tool.InputSchema != nil {
		schemaJSON, err := json.Marshal(tool.InputSchema)
		if err == nil {
			total += ot.CountTokens(string(schemaJSON))
		}
	}

	// Add overhead for tool structure (approximately 20 tokens)
	total += 20

	return total
}

// EstimateRequestTokens estimates total tokens for a request
func (ot *OfflineTokenizer) EstimateRequestTokens(req *types.AnthropicRequest) int {
	total := 0

	// Count system prompt
	if req.System != "" {
		total += ot.CountSystemTokens(req.System)
	}

	// Count system blocks
	if len(req.SystemBlocks) > 0 {
		total += ot.CountSystemBlocksTokens(req.SystemBlocks)
	}

	// Count tools
	for _, tool := range req.Tools {
		total += ot.CountToolTokens(tool)
	}

	// Count messages
	for _, message := range req.Messages {
		total += ot.CountMessageTokens(message)
	}

	// Add base overhead for request structure
	total += 10

	return total
}

// GetModelMinimumTokens returns the minimum cacheable tokens for a model
func (ot *OfflineTokenizer) GetModelMinimumTokens(model string) int {
	// Model-specific minimums based on Anthropic documentation
	if model == "claude-3-haiku-20240307" || model == "claude-3-5-haiku-20241022" {
		return 2048
	}
	return 1024
}

// GetTokenCountForCaching returns the token count with strategy multiplier applied
func (ot *OfflineTokenizer) GetTokenCountForCaching(text string, model string, strategy types.CacheStrategy) (int, int) {
	tokens := ot.CountTokens(text)
	minimumRequired := ot.GetModelMinimumTokens(model)

	// Apply strategy multiplier
	config := types.GetStrategyConfig(strategy)
	adjustedMinimum := int(float64(minimumRequired) * config.MinTokensMultiplier)

	return tokens, adjustedMinimum
}

// Tokenize returns the token IDs and their string representations
func (ot *OfflineTokenizer) Tokenize(text string) ([]uint32, []string, error) {
	if text == "" {
		return nil, nil, nil
	}

	ot.mu.RLock()
	defer ot.mu.RUnlock()

	// Encode the text
	encoding, err := ot.tokenizer.EncodeSingle(text)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to tokenize: %w", err)
	}

	// Convert IDs from int to uint32
	ids := make([]uint32, len(encoding.Ids))
	for i, id := range encoding.Ids {
		ids[i] = uint32(id)
	}

	// Get token strings
	tokenStrings := make([]string, len(encoding.Tokens))
	copy(tokenStrings, encoding.Tokens)

	return ids, tokenStrings, nil
}

// Helper functions for text analysis and logging

// sanitizeForLog removes control characters and newlines for safe logging
func sanitizeForLog(text string) string {
	result := make([]rune, 0, len(text))
	for _, r := range text {
		if r == '\n' || r == '\r' {
			result = append(result, ' ')
		} else if r >= 32 && r < 127 {
			// Printable ASCII
			result = append(result, r)
		} else if r >= 128 {
			// Unicode - keep it
			result = append(result, r)
		}
		// Skip other control characters
	}
	return string(result)
}

// hasNonASCII checks if text contains non-ASCII characters
func hasNonASCII(text string) bool {
	for _, r := range text {
		if r > 127 {
			return true
		}
	}
	return false
}
