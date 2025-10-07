package cache

import (
	"strings"
	"testing"
	"time"

	"autocache/internal/types"

	"github.com/sirupsen/logrus"
)

func TestNewCacheInjector(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel) // Suppress logs in tests

	// Use mock values for testing
	injector := NewCacheInjector(types.StrategyModerate, "https://api.anthropic.com", "test-key", logger)
	if injector == nil {
		t.Fatal("Expected cache injector to be created")
	}
}

func TestInjectCacheControl(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	tests := []struct {
		name              string
		strategy          types.CacheStrategy
		request           *types.AnthropicRequest
		expectInjected    bool
		expectBreakpoints int
		minCacheRatio     float64
	}{
		{
			name:     "System prompt with moderate strategy",
			strategy: types.StrategyModerate,
			request: &types.AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 100,
				System:    strings.Repeat("You are a helpful assistant with detailed instructions and context. ", 100), // ~6400 chars = ~4267 tokens
				Messages: []types.Message{
					{Role: "user", Content: []types.ContentBlock{{Type: "text", Text: "Hello"}}},
				},
			},
			expectInjected:    true,
			expectBreakpoints: 1,
			minCacheRatio:     0.5,
		},
		{
			name:     "Large system + tools with aggressive strategy",
			strategy: types.StrategyAggressive,
			request: &types.AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 100,
				System:    strings.Repeat("You are a helpful assistant with detailed instructions. ", 100),
				Tools: []types.ToolDefinition{
					{
						Name:        "calculator",
						Description: strings.Repeat("A tool for calculations. ", 50),
						InputSchema: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"expression": map[string]interface{}{
									"type":        "string",
									"description": "Mathematical expression to evaluate",
								},
							},
						},
					},
				},
				Messages: []types.Message{
					{Role: "user", Content: []types.ContentBlock{{Type: "text", Text: "Calculate 2+2"}}},
				},
			},
			expectInjected:    true,
			expectBreakpoints: 2,
			minCacheRatio:     0.6,
		},
		{
			name:     "Small content with conservative strategy",
			strategy: types.StrategyConservative,
			request: &types.AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 100,
				System:    "You are helpful.", // Too small
				Messages: []types.Message{
					{Role: "user", Content: []types.ContentBlock{{Type: "text", Text: "Hello"}}},
				},
			},
			expectInjected:    false,
			expectBreakpoints: 0,
			minCacheRatio:     0.0,
		},
		{
			name:     "Haiku model with large content",
			strategy: types.StrategyModerate,
			request: &types.AnthropicRequest{
				Model:     "claude-3-haiku-20240307",
				MaxTokens: 100,
				System:    strings.Repeat("You are a helpful assistant with very detailed instructions. ", 200), // Should exceed 2048 tokens
				Messages: []types.Message{
					{Role: "user", Content: []types.ContentBlock{{Type: "text", Text: "Help me"}}},
				},
			},
			expectInjected:    true,
			expectBreakpoints: 1,
			minCacheRatio:     0.8,
		},
		{
			name:     "Multiple large content blocks",
			strategy: types.StrategyAggressive,
			request: &types.AnthropicRequest{
				Model:     "claude-3-5-sonnet-20241022",
				MaxTokens: 100,
				System:    strings.Repeat("System instructions. ", 100),
				Messages: []types.Message{
					{
						Role: "user",
						Content: []types.ContentBlock{
							{Type: "text", Text: strings.Repeat("Here is a large document. ", 100)},
							{Type: "text", Text: strings.Repeat("Here is another large document. ", 100)},
						},
					},
				},
			},
			expectInjected:    true,
			expectBreakpoints: 3,
			minCacheRatio:     0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			injector := NewCacheInjector(tt.strategy, "https://api.anthropic.com", "test-key", logger)

			metadata, err := injector.InjectCacheControl(tt.request)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if metadata.CacheInjected != tt.expectInjected {
				t.Errorf("Expected cache injected %v, got %v", tt.expectInjected, metadata.CacheInjected)
			}

			if len(metadata.Breakpoints) != tt.expectBreakpoints {
				t.Errorf("Expected %d breakpoints, got %d", tt.expectBreakpoints, len(metadata.Breakpoints))
			}

			if metadata.CacheRatio < tt.minCacheRatio {
				t.Errorf("Expected cache ratio >= %.2f, got %.2f", tt.minCacheRatio, metadata.CacheRatio)
			}

			// Verify ROI calculations are reasonable
			if tt.expectInjected {
				if metadata.ROI.BreakEvenRequests <= 0 || metadata.ROI.BreakEvenRequests > 20 {
					t.Errorf("Unexpected break-even requests: %d", metadata.ROI.BreakEvenRequests)
				}
				if metadata.ROI.PercentSavings <= 0 {
					t.Errorf("Expected positive percent savings, got %.2f", metadata.ROI.PercentSavings)
				}
			}
		})
	}
}

func TestCollectCacheCandidates(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	injector := NewCacheInjector(types.StrategyModerate, "https://api.anthropic.com", "test-key", logger)

	request := &types.AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 100,
		System:    strings.Repeat("System prompt with detailed instructions. ", 150), // ~4500 chars = ~3000 tokens
		Tools: []types.ToolDefinition{
			{
				Name:        "test_tool",
				Description: strings.Repeat("Tool description with parameters and usage details. ", 100), // ~5200 chars = ~3467 tokens
			},
		},
		Messages: []types.Message{
			{
				Role: "user",
				Content: []types.ContentBlock{
					{Type: "text", Text: strings.Repeat("Large user message with context. ", 150)}, // ~5100 chars = ~3400 tokens
					{Type: "text", Text: "Small message"},
				},
			},
		},
	}

	config := types.GetStrategyConfig(types.StrategyModerate)
	minTokens := injector.GetTokenizer().GetModelMinimumTokens(request.Model)
	adjustedMinimum := int(float64(minTokens) * config.MinTokensMultiplier)

	candidates := injector.CollectCacheCandidates(request, adjustedMinimum, config)

	// Should find system, tools, and at least one large message content
	if len(candidates) < 3 {
		t.Errorf("Expected at least 3 cache candidates, got %d", len(candidates))
	}

	// Check that all candidates meet minimum token requirements
	for _, candidate := range candidates {
		if candidate.Tokens < adjustedMinimum {
			t.Errorf("Candidate %s has %d tokens, below minimum %d",
				candidate.Position, candidate.Tokens, adjustedMinimum)
		}
	}

	// Check that ROI scores are calculated
	for _, candidate := range candidates {
		if candidate.ROIScore <= 0 {
			t.Errorf("Candidate %s has invalid ROI score: %.2f",
				candidate.Position, candidate.ROIScore)
		}
	}
}

func TestCreateCandidate(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	injector := NewCacheInjector(types.StrategyModerate, "https://api.anthropic.com", "test-key", logger)

	tests := []struct {
		name        string
		position    string
		tokens      int
		contentType string
		ttl         string
		model       string
		expectHighROI bool
	}{
		{
			name:        "System prompt",
			position:    "system",
			tokens:      2000,
			contentType: "system",
			ttl:         "1h",
			model:       "claude-3-5-sonnet-20241022",
			expectHighROI: true,
		},
		{
			name:        "Tools",
			position:    "tools",
			tokens:      1500,
			contentType: "tools",
			ttl:         "1h",
			model:       "claude-3-5-sonnet-20241022",
			expectHighROI: true,
		},
		{
			name:        "Small content",
			position:    "message_0_block_0",
			tokens:      1024,
			contentType: "content",
			ttl:         "5m",
			model:       "claude-3-5-sonnet-20241022",
			expectHighROI: false,
		},
		{
			name:        "Large content",
			position:    "message_0_block_0",
			tokens:      5000,
			contentType: "content",
			ttl:         "5m",
			model:       "claude-3-5-sonnet-20241022",
			expectHighROI: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var content interface{} = "dummy content"
			candidate := injector.CreateCandidate(
				tt.position, tt.tokens, tt.contentType, tt.ttl, tt.model, &content)

			if candidate.Position != tt.position {
				t.Errorf("Expected position %s, got %s", tt.position, candidate.Position)
			}
			if candidate.Tokens != tt.tokens {
				t.Errorf("Expected %d tokens, got %d", tt.tokens, candidate.Tokens)
			}
			if candidate.ContentType != tt.contentType {
				t.Errorf("Expected content type %s, got %s", tt.contentType, candidate.ContentType)
			}
			if candidate.TTL != tt.ttl {
				t.Errorf("Expected TTL %s, got %s", tt.ttl, candidate.TTL)
			}

			// Verify ROI calculations
			if candidate.WriteCost <= 0 {
				t.Error("Write cost should be positive")
			}
			if candidate.ReadSavings <= 0 {
				t.Error("Read savings should be positive")
			}
			if candidate.BreakEven <= 0 {
				t.Error("Break even should be positive")
			}

			// System and tools should have higher ROI scores than regular content (> 0.5)
			if tt.expectHighROI && candidate.ROIScore < 0.5 {
				t.Errorf("Expected high ROI score for %s, got %.2f", tt.contentType, candidate.ROIScore)
			}
		})
	}
}

func TestCalculateROIScore(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	injector := NewCacheInjector(types.StrategyModerate, "https://api.anthropic.com", "test-key", logger)

	tests := []struct {
		name           string
		tokens         int
		writeCost      float64
		readSavings    float64
		breakEven      int
		contentType    string
		expectedHigher bool // Whether this should score higher than baseline
	}{
		{
			name:           "System content",
			tokens:         2000,
			writeCost:      0.01,
			readSavings:    0.005,
			breakEven:      2,
			contentType:    "system",
			expectedHigher: true,
		},
		{
			name:           "Tools content",
			tokens:         1500,
			writeCost:      0.008,
			readSavings:    0.004,
			breakEven:      2,
			contentType:    "tools",
			expectedHigher: true,
		},
		{
			name:           "Regular content",
			tokens:         1000,
			writeCost:      0.005,
			readSavings:    0.0025,
			breakEven:      2,
			contentType:    "content",
			expectedHigher: false,
		},
		{
			name:           "Large content",
			tokens:         5000,
			writeCost:      0.02,
			readSavings:    0.01,
			breakEven:      2,
			contentType:    "content",
			expectedHigher: true,
		},
		{
			name:           "High break-even content",
			tokens:         1000,
			writeCost:      0.01,
			readSavings:    0.001,
			breakEven:      15,
			contentType:    "content",
			expectedHigher: false,
		},
	}

	baselineScore := injector.CalculateROIScore(1000, 0.005, 0.0025, 2, "content")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := injector.CalculateROIScore(
				tt.tokens, tt.writeCost, tt.readSavings, tt.breakEven, tt.contentType)

			if score <= 0 {
				t.Error("ROI score should be positive")
			}

			if tt.expectedHigher && score <= baselineScore {
				t.Errorf("Expected score %.2f to be higher than baseline %.2f",
					score, baselineScore)
			}
		})
	}
}

func TestDetermineTTLForContent(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	injector := NewCacheInjector(types.StrategyModerate, "https://api.anthropic.com", "test-key", logger)
	config := types.GetStrategyConfig(types.StrategyModerate)

	tests := []struct {
		name        string
		text        string
		expectedTTL string
	}{
		{
			name:        "Stable system-like content",
			text:        strings.Repeat("You are a helpful assistant. Instructions: follow these guidelines carefully. ", 50),
			expectedTTL: "1h",
		},
		{
			name:        "Regular content",
			text:        "This is just a regular user message without any special patterns.",
			expectedTTL: config.ContentTTL,
		},
		{
			name:        "Short stable content",
			text:        "You are helpful.", // Too short for 1h TTL
			expectedTTL: config.ContentTTL,
		},
		{
			name:        "Context-heavy content",
			text:        strings.Repeat("Context: this is important background information. ", 30),
			expectedTTL: "1h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ttl := injector.DetermineTTLForContent(tt.text, config)
			if ttl != tt.expectedTTL {
				t.Errorf("DetermineTTLForContent() = %s, expected %s", ttl, tt.expectedTTL)
			}
		})
	}
}

func TestApplyCacheControl(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	injector := NewCacheInjector(types.StrategyModerate, "https://api.anthropic.com", "test-key", logger)

	// Create test content blocks
	systemContent := "System instructions"
	contentBlock := types.ContentBlock{Type: "text", Text: "User message"}
	toolDef := types.ToolDefinition{Name: "test_tool", Description: "Test tool"}

	candidates := []CacheCandidate{
		{
			Position:    "system",
			Tokens:      1000,
			ContentType: "system",
			TTL:         "1h",
			WriteCost:   0.01,
			ReadSavings: 0.005,
			Content:     &systemContent,
		},
		{
			Position:    "message_0_block_0",
			Tokens:      1500,
			ContentType: "content",
			TTL:         "5m",
			WriteCost:   0.008,
			ReadSavings: 0.004,
			Content:     &contentBlock,
		},
		{
			Position:    "tools",
			Tokens:      1200,
			ContentType: "tools",
			TTL:         "1h",
			WriteCost:   0.006,
			ReadSavings: 0.003,
			Content:     &[]types.ToolDefinition{toolDef},
		},
	}

	breakpoints := injector.ApplyCacheControl(candidates)

	if len(breakpoints) != len(candidates) {
		t.Errorf("Expected %d breakpoints, got %d", len(candidates), len(breakpoints))
	}

	for i, bp := range breakpoints {
		if bp.Position != candidates[i].Position {
			t.Errorf("Breakpoint %d position mismatch: got %s, expected %s",
				i, bp.Position, candidates[i].Position)
		}
		if bp.Tokens != candidates[i].Tokens {
			t.Errorf("Breakpoint %d tokens mismatch: got %d, expected %d",
				i, bp.Tokens, candidates[i].Tokens)
		}
		if bp.TTL != candidates[i].TTL {
			t.Errorf("Breakpoint %d TTL mismatch: got %s, expected %s",
				i, bp.TTL, candidates[i].TTL)
		}
	}

	// Check that cache control was actually applied to the content block
	if contentBlock.CacheControl == nil {
		t.Error("Cache control was not applied to content block")
	} else {
		if contentBlock.CacheControl.Type != "ephemeral" {
			t.Errorf("Expected cache control type 'ephemeral', got %s",
				contentBlock.CacheControl.Type)
		}
		if contentBlock.CacheControl.TTL != "5m" {
			t.Errorf("Expected cache control TTL '5m', got %s",
				contentBlock.CacheControl.TTL)
		}
	}
}

func TestCalculateMetadata(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	injector := NewCacheInjector(types.StrategyModerate, "https://api.anthropic.com", "test-key", logger)

	request := &types.AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 100,
		System:    strings.Repeat("System. ", 100),
		Messages: []types.Message{
			{Role: "user", Content: []types.ContentBlock{{Type: "text", Text: "Hello"}}},
		},
	}

	breakpoints := []types.CacheBreakpoint{
		{
			Position:    "system",
			Tokens:      500,
			TTL:         "1h",
			Type:        "system",
			WritePrice:  0.01,
			ReadSavings: 0.005,
			Timestamp:   time.Now(),
		},
	}

	startTime := time.Now()
	metadata := injector.calculateMetadata(request, breakpoints, startTime)

	if metadata == nil {
		t.Fatal("Expected metadata to be calculated")
	}

	if !metadata.CacheInjected {
		t.Error("Expected cache injected to be true")
	}

	if metadata.TotalTokens <= 0 {
		t.Error("Expected positive total tokens")
	}

	if metadata.CachedTokens != 500 {
		t.Errorf("Expected 500 cached tokens, got %d", metadata.CachedTokens)
	}

	expectedRatio := float64(500) / float64(metadata.TotalTokens)
	if metadata.CacheRatio != expectedRatio {
		t.Errorf("Expected cache ratio %.3f, got %.3f", expectedRatio, metadata.CacheRatio)
	}

	if len(metadata.Breakpoints) != 1 {
		t.Errorf("Expected 1 breakpoint in metadata, got %d", len(metadata.Breakpoints))
	}

	if metadata.Strategy != string(types.StrategyModerate) {
		t.Errorf("Expected strategy %s, got %s", types.StrategyModerate, metadata.Strategy)
	}

	if metadata.Model != request.Model {
		t.Errorf("Expected model %s, got %s", request.Model, metadata.Model)
	}
}

func TestContainsCaseInsensitive(t *testing.T) {
	tests := []struct {
		text     string
		substr   string
		expected bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"Hello World", "HeLLo WoRLd", true},
		{"Hello World", "xyz", false},
		{"", "test", false},
		{"test", "", false},
		{"You are a helpful assistant", "You are", true},
		{"Instructions: follow these", "INSTRUCTIONS", true},
	}

	for _, tt := range tests {
		t.Run(tt.text+" contains "+tt.substr, func(t *testing.T) {
			result := containsCaseInsensitive(tt.text, tt.substr)
			if result != tt.expected {
				t.Errorf("containsCaseInsensitive(%q, %q) = %v, expected %v",
					tt.text, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestStrategyBehavior(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Create a request with multiple cacheable elements
	request := &types.AnthropicRequest{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 100,
		System:    strings.Repeat("System instructions. ", 100),
		Tools: []types.ToolDefinition{
			{Name: "tool1", Description: strings.Repeat("Tool description. ", 50)},
		},
		Messages: []types.Message{
			{
				Role: "user",
				Content: []types.ContentBlock{
					{Type: "text", Text: strings.Repeat("Large content block 1. ", 100)},
					{Type: "text", Text: strings.Repeat("Large content block 2. ", 100)},
				},
			},
		},
	}

	strategies := []types.CacheStrategy{types.StrategyConservative, types.StrategyModerate, types.StrategyAggressive}
	var results []int

	for _, strategy := range strategies {
		injector := NewCacheInjector(strategy, "https://api.anthropic.com", "test-key", logger)
		metadata, err := injector.InjectCacheControl(request)
		if err != nil {
			t.Errorf("Strategy %s failed: %v", strategy, err)
			continue
		}
		results = append(results, len(metadata.Breakpoints))
	}

	if len(results) != 3 {
		t.Fatal("Expected results for all 3 strategies")
	}

	// Conservative should use fewer breakpoints than moderate
	if results[0] > results[1] {
		t.Errorf("Conservative strategy used more breakpoints (%d) than moderate (%d)",
			results[0], results[1])
	}

	// Moderate should use fewer or equal breakpoints than aggressive
	if results[1] > results[2] {
		t.Errorf("Moderate strategy used more breakpoints (%d) than aggressive (%d)",
			results[1], results[2])
	}
}