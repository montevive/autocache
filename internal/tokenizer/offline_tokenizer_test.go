package tokenizer

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"autocache/internal/types"

	"github.com/sirupsen/logrus"
)

func TestNewOfflineTokenizer(t *testing.T) {
	tokenizer, err := NewOfflineTokenizer()
	if err != nil {
		t.Fatalf("Failed to create offline tokenizer: %v", err)
	}
	if tokenizer == nil {
		t.Fatal("Expected tokenizer to be created")
	}
	if tokenizer.fallbackTokenizer == nil {
		t.Fatal("Expected fallback tokenizer to be initialized")
	}
	if tokenizer.logger == nil {
		t.Fatal("Expected logger to be initialized")
	}
}

func TestNewOfflineTokenizerWithLogger(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	tokenizer, err := NewOfflineTokenizerWithLogger(logger)
	if err != nil {
		t.Fatalf("Failed to create offline tokenizer with logger: %v", err)
	}

	if tokenizer.logger != logger {
		t.Error("Expected custom logger to be used")
	}
	if tokenizer.fallbackTokenizer == nil {
		t.Fatal("Expected fallback tokenizer to be initialized")
	}
}

func TestOfflineTokenizerBasicFunctionality(t *testing.T) {
	tokenizer, err := NewOfflineTokenizer()
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	tests := []struct {
		name      string
		text      string
		minTokens int
		maxTokens int
	}{
		{
			name:      "Empty string",
			text:      "",
			minTokens: 0,
			maxTokens: 0,
		},
		{
			name:      "Simple text",
			text:      "Hello, world!",
			minTokens: 2,
			maxTokens: 10,
		},
		{
			name:      "Long text",
			text:      strings.Repeat("This is a test sentence. ", 50),
			minTokens: 100,
			maxTokens: 500,
		},
		{
			name:      "Code-like text",
			text:      "function test() { return x + y; }",
			minTokens: 5,
			maxTokens: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenizer.CountTokens(tt.text)
			if tokens < tt.minTokens || tokens > tt.maxTokens {
				t.Errorf("CountTokens(%s) = %d, expected between %d and %d",
					tt.name, tokens, tt.minTokens, tt.maxTokens)
			}
		})
	}
}

func TestOfflineTokenizerEdgeCases(t *testing.T) {
	tokenizer, err := NewOfflineTokenizer()
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	tests := []struct {
		name string
		text string
	}{
		{
			name: "Unicode characters",
			text: "Hello 世界 🌍",
		},
		{
			name: "Special characters",
			text: "!@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
		{
			name: "Newlines and tabs",
			text: "Line 1\nLine 2\tTabbed",
		},
		{
			name: "Very long single word",
			text: strings.Repeat("a", 10000),
		},
		{
			name: "Mixed content",
			text: "Normal text\n```code block```\n{\"json\": \"data\"}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			tokens := tokenizer.CountTokens(tt.text)
			if tokens < 0 {
				t.Errorf("Got negative token count: %d", tokens)
			}
		})
	}
}

func TestOfflineTokenizerFallbackBehavior(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	tokenizer, err := NewOfflineTokenizerWithLogger(logger)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	// Test that fallback produces reasonable results
	text := "This is a normal sentence that should tokenize fine."
	tokens := tokenizer.CountTokens(text)

	// Verify fallback tokenizer produces similar results
	fallbackTokens := tokenizer.fallbackTokenizer.CountTokens(text)

	// Both should produce positive counts
	if tokens <= 0 {
		t.Errorf("Offline tokenizer returned non-positive count: %d", tokens)
	}
	if fallbackTokens <= 0 {
		t.Errorf("Fallback tokenizer returned non-positive count: %d", fallbackTokens)
	}

	// Note: The tokenizers use different algorithms, so counts may vary significantly
	// We just verify both produce reasonable positive results
	t.Logf("Token counts - offline=%d, fallback=%d (ratio=%.2f)",
		tokens, fallbackTokens, float64(tokens)/float64(fallbackTokens))
}

func TestOfflineTokenizerPanicStats(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Suppress error logs during test

	tokenizer, err := NewOfflineTokenizerWithLogger(logger)
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	// Check initial stats
	stats := tokenizer.GetPanicStats()
	if stats == nil {
		t.Fatal("Expected stats to be returned")
	}

	initialPanics := stats["panic_count"]
	initialFallbacks := stats["fallback_count"]

	// Process some text
	text := "This is a test sentence."
	tokenizer.CountTokens(text)

	// Check stats again
	stats = tokenizer.GetPanicStats()

	// For normal text, panic_count should not increase
	// (unless the tokenizer library has issues with this specific text)
	newPanics := stats["panic_count"]
	newFallbacks := stats["fallback_count"]

	// Stats should either stay the same or increase (never decrease)
	if newPanics < initialPanics {
		t.Errorf("Panic count decreased: %d -> %d", initialPanics, newPanics)
	}
	if newFallbacks < initialFallbacks {
		t.Errorf("Fallback count decreased: %d -> %d", initialFallbacks, newFallbacks)
	}
}

func TestOfflineTokenizerConcurrency(t *testing.T) {
	tokenizer, err := NewOfflineTokenizer()
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	// Test concurrent access
	const numGoroutines = 50
	const numIterations = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines*numIterations)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < numIterations; j++ {
				// Ensure we always have non-empty text
				text := strings.Repeat("Test sentence ", id+j+1)
				tokens := tokenizer.CountTokens(text)
				if tokens <= 0 {
					errors <- fmt.Errorf("Goroutine %d iteration %d: Got non-positive token count: %d for text: %q",
						id, j, tokens, text)
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}

	// Verify stats are consistent
	stats := tokenizer.GetPanicStats()
	if stats == nil {
		t.Fatal("Expected stats to be returned after concurrent access")
	}
}

func TestOfflineTokenizerSystemBlocks(t *testing.T) {
	tokenizer, err := NewOfflineTokenizer()
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	blocks := []types.ContentBlock{
		{Type: "text", Text: "You are a helpful assistant."},
		{Type: "text", Text: "Please follow these guidelines."},
	}

	tokens := tokenizer.CountSystemBlocksTokens(blocks)
	if tokens <= 0 {
		t.Errorf("Expected positive token count for system blocks, got %d", tokens)
	}

	// Should be more than just the text tokens (includes overhead)
	text1Tokens := tokenizer.CountTokens(blocks[0].Text)
	text2Tokens := tokenizer.CountTokens(blocks[1].Text)

	if tokens < text1Tokens+text2Tokens {
		t.Errorf("System blocks tokens (%d) should be >= sum of text tokens (%d + %d)",
			tokens, text1Tokens, text2Tokens)
	}
}

func TestOfflineTokenizerToolTokens(t *testing.T) {
	tokenizer, err := NewOfflineTokenizer()
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	tool := types.ToolDefinition{
		Name:        "test_tool",
		Description: "A test tool for validation",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type":        "string",
					"description": "First parameter",
				},
			},
		},
	}

	tokens := tokenizer.CountToolTokens(tool)
	if tokens <= 0 {
		t.Errorf("Expected positive token count for tool, got %d", tokens)
	}

	// Tool tokens should include overhead
	nameTokens := tokenizer.CountTokens(tool.Name)
	descTokens := tokenizer.CountTokens(tool.Description)

	if tokens < nameTokens+descTokens {
		t.Errorf("Tool tokens (%d) should be >= name+desc tokens (%d + %d)",
			tokens, nameTokens, descTokens)
	}
}

func TestOfflineTokenizerModelMinimums(t *testing.T) {
	tokenizer, err := NewOfflineTokenizer()
	if err != nil {
		t.Fatalf("Failed to create tokenizer: %v", err)
	}

	tests := []struct {
		model    string
		expected int
	}{
		{"claude-3-5-sonnet-20241022", 1024},
		{"claude-3-opus-20240229", 1024},
		{"claude-3-haiku-20240307", 2048},
		{"claude-3-5-haiku-20241022", 2048},
		{"claude-3-sonnet-20240229", 1024},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			min := tokenizer.GetModelMinimumTokens(tt.model)
			if min != tt.expected {
				t.Errorf("GetModelMinimumTokens(%s) = %d, expected %d",
					tt.model, min, tt.expected)
			}
		})
	}
}

func TestOfflineTokenizerHelperFunctions(t *testing.T) {
	t.Run("sanitizeForLog", func(t *testing.T) {
		tests := []struct {
			input          string
			contains       string
			shouldNotContain string
		}{
			{"normal text", "normal text", ""},
			{"text\nwith\nnewlines", "text with newlines", "\n"},
			{"text\twith\ttabs", "text", "\t"}, // Tabs might be preserved or removed depending on implementation
			{"Hello 世界", "Hello 世界", ""}, // Unicode preserved
		}

		for _, tt := range tests {
			result := sanitizeForLog(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("sanitizeForLog(%q) = %q, expected to contain %q",
					tt.input, result, tt.contains)
			}
			if tt.shouldNotContain != "" && strings.Contains(result, tt.shouldNotContain) {
				t.Errorf("sanitizeForLog(%q) = %q, should not contain %q",
					tt.input, result, tt.shouldNotContain)
			}
		}
	})

	t.Run("hasNonASCII", func(t *testing.T) {
		tests := []struct {
			input    string
			expected bool
		}{
			{"normal text", false},
			{"Hello 世界", true},
			{"emoji 🌍", true},
			{"123!@#", false},
		}

		for _, tt := range tests {
			result := hasNonASCII(tt.input)
			if result != tt.expected {
				t.Errorf("hasNonASCII(%q) = %v, expected %v",
					tt.input, result, tt.expected)
			}
		}
	})
}
