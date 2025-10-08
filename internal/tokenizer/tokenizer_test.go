package tokenizer

import (
	"strings"
	"testing"

	"autocache/internal/types"
)

func TestNewAnthropicTokenizer(t *testing.T) {
	tok := NewAnthropicTokenizer()
	if tok == nil {
		t.Fatal("Expected tokenizer to be created")
	}
}

func TestCountTokens(t *testing.T) {
	tok := NewAnthropicTokenizer()

	tests := []struct {
		name     string
		text     string
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
			name:      "Short text",
			text:      "Hello world",
			minTokens: 5,  // ~11 chars / 1.5 = 7.3, adjusted for short text
			maxTokens: 12,
		},
		{
			name:      "Medium text",
			text:      "This is a longer piece of text that should have more tokens than the short text above.",
			minTokens: 50,  // ~87 chars / 1.5 = 58
			maxTokens: 70,
		},
		{
			name:      "Code-like text",
			text:      "function calculate(x, y) { return x + y; }",
			minTokens: 30,  // Code gets 1.2x multiplier
			maxTokens: 45,
		},
		{
			name:      "JSON-like text",
			text:      `{"key": "value", "number": 123, "array": [1, 2, 3]}`,
			minTokens: 45,  // JSON gets 1.3x multiplier
			maxTokens: 65,
		},
		{
			name:      "Very long text",
			text:      strings.Repeat("This is a repeating sentence. ", 100),
			minTokens: 1900, // ~3000 chars / 1.5 = 2000 + overhead
			maxTokens: 2100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tok.CountTokens(tt.text)
			if tokens < tt.minTokens || tokens > tt.maxTokens {
				t.Errorf("CountTokens(%s) = %d, expected between %d and %d",
					tt.name, tokens, tt.minTokens, tt.maxTokens)
			}

			// Test caching
			tokens2 := tok.CountTokens(tt.text)
			if tokens != tokens2 {
				t.Errorf("Cached token count mismatch: %d != %d", tokens, tokens2)
			}
		})
	}
}

func TestCountMessageTokens(t *testing.T) {
	tok := NewAnthropicTokenizer()

	tests := []struct {
		name     string
		message  types.Message
		minTokens int
	}{
		{
			name: "Simple user message",
			message: types.Message{
				Role: "user",
				Content: []types.ContentBlock{
					{Type: "text", Text: "Hello, how are you?"},
				},
			},
			minTokens: 5,
		},
		{
			name: "Multi-block message",
			message: types.Message{
				Role: "assistant",
				Content: []types.ContentBlock{
					{Type: "text", Text: "First block"},
					{Type: "text", Text: "Second block"},
				},
			},
			minTokens: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tok.CountMessageTokens(tt.message)
			if tokens < tt.minTokens {
				t.Errorf("CountMessageTokens(%s) = %d, expected at least %d",
					tt.name, tokens, tt.minTokens)
			}
		})
	}
}

func TestCountContentBlockTokens(t *testing.T) {
	tok := NewAnthropicTokenizer()

	tests := []struct {
		name     string
		block    types.ContentBlock
		minTokens int
	}{
		{
			name:      "Text block",
			block:     types.ContentBlock{Type: "text", Text: "This is a text block"},
			minTokens: 5,
		},
		{
			name:      "Empty text block",
			block:     types.ContentBlock{Type: "text", Text: ""},
			minTokens: 2, // Still has overhead
		},
		{
			name:      "Image block",
			block:     types.ContentBlock{Type: "image"},
			minTokens: 85, // Base image token cost
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tok.CountContentBlockTokens(tt.block)
			if tokens < tt.minTokens {
				t.Errorf("CountContentBlockTokens(%s) = %d, expected at least %d",
					tt.name, tokens, tt.minTokens)
			}
		})
	}
}

func TestCountToolTokens(t *testing.T) {
	tok := NewAnthropicTokenizer()

	tool := types.ToolDefinition{
		Name:        "get_weather",
		Description: "Get the current weather for a location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location": map[string]interface{}{
					"type":        "string",
					"description": "The city and state",
				},
			},
		},
	}

	tokens := tok.CountToolTokens(tool)
	if tokens < 20 {
		t.Errorf("CountToolTokens() = %d, expected at least 20", tokens)
	}
}

func TestGetModelMinimumTokens(t *testing.T) {
	tok := NewAnthropicTokenizer()

	tests := []struct {
		model    string
		expected int
	}{
		{"claude-3-5-sonnet-20241022", 1024},
		{"claude-3-opus-20240229", 1024},
		{"claude-3-haiku-20240307", 2048},
		{"claude-3-5-haiku-20241022", 2048},
		{"unknown-model", 1024},
		{"some-haiku-variant", 2048},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			minimum := tok.GetModelMinimumTokens(tt.model)
			if minimum != tt.expected {
				t.Errorf("GetModelMinimumTokens(%s) = %d, expected %d",
					tt.model, minimum, tt.expected)
			}
		})
	}
}

func TestCountSystemTokens(t *testing.T) {
	tok := NewAnthropicTokenizer()

	tests := []struct {
		name     string
		system   string
		minTokens int
	}{
		{
			name:      "Empty system",
			system:    "",
			minTokens: 0,
		},
		{
			name:      "Short system prompt",
			system:    "You are a helpful assistant.",
			minTokens: 7,
		},
		{
			name:      "Long system prompt",
			system:    strings.Repeat("You must follow these instructions carefully. ", 20),
			minTokens: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tok.CountSystemTokens(tt.system)
			if tokens < tt.minTokens {
				t.Errorf("CountSystemTokens(%s) = %d, expected at least %d",
					tt.name, tokens, tt.minTokens)
			}
		})
	}
}

func TestEstimateRequestTokens(t *testing.T) {
	tok := NewAnthropicTokenizer()

	req := &types.AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 100,
		System:    "You are a helpful assistant.",
		Messages: []types.Message{
			{
				Role: "user",
				Content: []types.ContentBlock{
					{Type: "text", Text: "Hello, how can you help me?"},
				},
			},
		},
		Tools: []types.ToolDefinition{
			{
				Name:        "calculator",
				Description: "Perform calculations",
			},
		},
	}

	tokens := tok.EstimateRequestTokens(req)
	if tokens < 30 {
		t.Errorf("EstimateRequestTokens() = %d, expected at least 30", tokens)
	}
}

func TestGetTokenCountForCaching(t *testing.T) {
	tok := NewAnthropicTokenizer()

	tests := []struct {
		name     string
		text     string
		model    string
		strategy types.CacheStrategy
		wantMinimumHigher bool
	}{
		{
			name:     "Conservative strategy increases minimum",
			text:     strings.Repeat("test ", 500),
			model:    "claude-3-5-sonnet-20241022",
			strategy: types.StrategyConservative,
			wantMinimumHigher: true,
		},
		{
			name:     "Aggressive strategy decreases minimum",
			text:     strings.Repeat("test ", 500),
			model:    "claude-3-5-sonnet-20241022",
			strategy: types.StrategyAggressive,
			wantMinimumHigher: false,
		},
		{
			name:     "Haiku model has higher base minimum",
			text:     strings.Repeat("test ", 500),
			model:    "claude-3-haiku-20240307",
			strategy: types.StrategyModerate,
			wantMinimumHigher: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, minimum := tok.GetTokenCountForCaching(tt.text, tt.model, tt.strategy)

			if tokens <= 0 {
				t.Errorf("Expected positive token count, got %d", tokens)
			}

			baseMinimum := tok.GetModelMinimumTokens(tt.model)
			if tt.strategy == types.StrategyConservative && minimum <= baseMinimum {
				t.Errorf("Conservative strategy should increase minimum, got %d <= %d",
					minimum, baseMinimum)
			}
			if tt.strategy == types.StrategyAggressive && minimum >= baseMinimum {
				t.Errorf("Aggressive strategy should decrease minimum, got %d >= %d",
					minimum, baseMinimum)
			}
		})
	}
}
