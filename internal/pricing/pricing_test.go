package pricing

import (
	"math"
	"testing"

	"autocache/internal/types"
)

func TestNewPricingCalculator(t *testing.T) {
	calc := NewPricingCalculator()
	if calc == nil {
		t.Fatal("Expected pricing calculator to be created")
	}
	if len(calc.models) == 0 {
		t.Fatal("Expected models to be loaded")
	}
}

func TestGetModelPricing(t *testing.T) {
	calc := NewPricingCalculator()

	tests := []struct {
		name          string
		model         string
		expectError   bool
		expectedInput float64
	}{
		{
			name:          "Claude 3.5 Sonnet latest",
			model:         "claude-3-5-sonnet-20241022",
			expectError:   false,
			expectedInput: 3.00,
		},
		{
			name:          "Claude 3 Haiku",
			model:         "claude-3-haiku-20240307",
			expectError:   false,
			expectedInput: 0.25,
		},
		{
			name:          "Claude 3 Opus",
			model:         "claude-3-opus-20240229",
			expectError:   false,
			expectedInput: 15.00,
		},
		{
			name:          "Unknown model defaults to Sonnet",
			model:         "unknown-model-xyz",
			expectError:   true,
			expectedInput: 3.00, // Default to Sonnet pricing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing, err := calc.GetModelPricing(tt.model)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if pricing.InputTokens != tt.expectedInput {
				t.Errorf("GetModelPricing(%s) input price = %.2f, expected %.2f",
					tt.model, pricing.InputTokens, tt.expectedInput)
			}

			// Verify cache pricing relationships (with floating point tolerance)
			if math.Abs(pricing.CacheWrite5m-pricing.InputTokens*1.25) > 0.001 {
				t.Errorf("5m cache write price should be 1.25x input price, got %.3f, expected %.3f",
					pricing.CacheWrite5m, pricing.InputTokens*1.25)
			}
			if math.Abs(pricing.CacheWrite1h-pricing.InputTokens*2.0) > 0.001 {
				t.Errorf("1h cache write price should be 2x input price, got %.3f, expected %.3f",
					pricing.CacheWrite1h, pricing.InputTokens*2.0)
			}
			if math.Abs(pricing.CacheRead-pricing.InputTokens*0.1) > 0.001 {
				t.Errorf("Cache read price should be 0.1x input price, got %.3f, expected %.3f",
					pricing.CacheRead, pricing.InputTokens*0.1)
			}
		})
	}
}

func TestCalculateBaseCost(t *testing.T) {
	calc := NewPricingCalculator()

	tests := []struct {
		name         string
		model        string
		inputTokens  int
		outputTokens int
		expectedCost float64
	}{
		{
			name:         "Small Sonnet request",
			model:        "claude-3-5-sonnet-20241022",
			inputTokens:  1000,
			outputTokens: 500,
			expectedCost: 0.003 + 0.0075, // $3/1M * 1K + $15/1M * 0.5K
		},
		{
			name:         "Large Haiku request",
			model:        "claude-3-haiku-20240307",
			inputTokens:  10000,
			outputTokens: 2000,
			expectedCost: 0.0025 + 0.0025, // $0.25/1M * 10K + $1.25/1M * 2K
		},
		{
			name:         "Opus request",
			model:        "claude-3-opus-20240229",
			inputTokens:  5000,
			outputTokens: 1000,
			expectedCost: 0.075 + 0.075, // $15/1M * 5K + $75/1M * 1K
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost, err := calc.CalculateBaseCost(tt.model, tt.inputTokens, tt.outputTokens)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if math.Abs(cost-tt.expectedCost) > 0.00001 {
				t.Errorf("CalculateBaseCost(%s, %d, %d) = %.6f, expected %.6f",
					tt.model, tt.inputTokens, tt.outputTokens, cost, tt.expectedCost)
			}
		})
	}
}

func TestCalculateCacheWriteCost(t *testing.T) {
	calc := NewPricingCalculator()

	tests := []struct {
		name         string
		model        string
		tokens       int
		ttl          string
		expectedCost float64
	}{
		{
			name:         "Sonnet 5m cache",
			model:        "claude-3-5-sonnet-20241022",
			tokens:       1000,
			ttl:          "5m",
			expectedCost: 0.00375, // $3.75/1M * 1K
		},
		{
			name:         "Sonnet 1h cache",
			model:        "claude-3-5-sonnet-20241022",
			tokens:       1000,
			ttl:          "1h",
			expectedCost: 0.006, // $6/1M * 1K
		},
		{
			name:         "Haiku 5m cache",
			model:        "claude-3-haiku-20240307",
			tokens:       2048,
			ttl:          "5m",
			expectedCost: 0.00064, // $0.3125/1M * 2048
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost, err := calc.CalculateCacheWriteCost(tt.model, tt.tokens, tt.ttl)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if math.Abs(cost-tt.expectedCost) > 0.00001 {
				t.Errorf("CalculateCacheWriteCost(%s, %d, %s) = %.6f, expected %.6f",
					tt.model, tt.tokens, tt.ttl, cost, tt.expectedCost)
			}
		})
	}
}

func TestCalculateCacheReadCost(t *testing.T) {
	calc := NewPricingCalculator()

	tests := []struct {
		name         string
		model        string
		tokens       int
		expectedCost float64
	}{
		{
			name:         "Sonnet cache read",
			model:        "claude-3-5-sonnet-20241022",
			tokens:       1000,
			expectedCost: 0.0003, // $0.30/1M * 1K
		},
		{
			name:         "Haiku cache read",
			model:        "claude-3-haiku-20240307",
			tokens:       2048,
			expectedCost: 0.0000512, // $0.025/1M * 2048
		},
		{
			name:         "Opus cache read",
			model:        "claude-3-opus-20240229",
			tokens:       5000,
			expectedCost: 0.0075, // $1.50/1M * 5K
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost, err := calc.CalculateCacheReadCost(tt.model, tt.tokens)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if math.Abs(cost-tt.expectedCost) > 0.00001 {
				t.Errorf("CalculateCacheReadCost(%s, %d) = %.6f, expected %.6f",
					tt.model, tt.tokens, cost, tt.expectedCost)
			}
		})
	}
}

func TestCalculateROI(t *testing.T) {
	calc := NewPricingCalculator()

	tests := []struct {
		name              string
		model             string
		totalTokens       int
		cachedTokens      int
		breakpoints       []types.CacheBreakpoint
		expectedBreakEven int
		expectPositiveROI bool
	}{
		{
			name:         "High cache ratio",
			model:        "claude-3-5-sonnet-20241022",
			totalTokens:  10000,
			cachedTokens: 8000,
			breakpoints: []types.CacheBreakpoint{
				{Tokens: 8000, TTL: "1h"},
			},
			expectedBreakEven: 2,
			expectPositiveROI: true,
		},
		{
			name:         "Low cache ratio",
			model:        "claude-3-5-sonnet-20241022",
			totalTokens:  10000,
			cachedTokens: 1000,
			breakpoints: []types.CacheBreakpoint{
				{Tokens: 1000, TTL: "5m"},
			},
			expectedBreakEven: 2,
			expectPositiveROI: true,
		},
		{
			name:         "Multiple breakpoints",
			model:        "claude-3-haiku-20240307",
			totalTokens:  20000,
			cachedTokens: 15000,
			breakpoints: []types.CacheBreakpoint{
				{Tokens: 5000, TTL: "1h"},
				{Tokens: 5000, TTL: "1h"},
				{Tokens: 5000, TTL: "5m"},
			},
			expectedBreakEven: 2,
			expectPositiveROI: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roi, err := calc.CalculateROI(tt.model, tt.totalTokens, tt.cachedTokens, tt.breakpoints)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Check break-even is reasonable
			if roi.BreakEvenRequests < 1 || roi.BreakEvenRequests > 10 {
				t.Errorf("Unexpected break-even requests: %d", roi.BreakEvenRequests)
			}

			// Check percentage savings is positive for cached content
			if tt.expectPositiveROI && roi.PercentSavings <= 0 {
				t.Errorf("Expected positive ROI percentage, got %.2f", roi.PercentSavings)
			}

			// Check that subsequent savings is positive
			if tt.expectPositiveROI && roi.SubsequentSavings <= 0 {
				t.Errorf("Expected positive subsequent savings, got %.6f", roi.SubsequentSavings)
			}

			// Check that savings increase with more requests
			if roi.SavingsAt100Requests <= roi.SavingsAt10Requests {
				t.Errorf("Savings at 100 requests (%.6f) should be greater than at 10 requests (%.6f)",
					roi.SavingsAt100Requests, roi.SavingsAt10Requests)
			}
		})
	}
}

func TestEstimateBreakpointROI(t *testing.T) {
	calc := NewPricingCalculator()

	tests := []struct {
		name              string
		model             string
		tokens            int
		ttl               string
		expectBreakEven   int
		expectPositiveSavings bool
	}{
		{
			name:              "Large content with 1h cache",
			model:             "claude-3-5-sonnet-20241022",
			tokens:            5000,
			ttl:               "1h",
			expectBreakEven:   3, // 1h cache costs 2x, needs more requests to break even
			expectPositiveSavings: true,
		},
		{
			name:              "Small content with 5m cache",
			model:             "claude-3-5-sonnet-20241022",
			tokens:            1024,
			ttl:               "5m",
			expectBreakEven:   2,
			expectPositiveSavings: true,
		},
		{
			name:              "Haiku with minimum tokens",
			model:             "claude-3-haiku-20240307",
			tokens:            2048,
			ttl:               "5m",
			expectBreakEven:   2,
			expectPositiveSavings: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeCost, savingsPerRead, breakEven, err := calc.EstimateBreakpointROI(
				tt.model, tt.tokens, tt.ttl)

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if writeCost <= 0 {
				t.Error("Write cost should be positive")
			}

			if tt.expectPositiveSavings && savingsPerRead <= 0 {
				t.Errorf("Expected positive savings per read, got %.6f", savingsPerRead)
			}

			if breakEven != tt.expectBreakEven {
				t.Errorf("EstimateBreakpointROI() break-even = %d, expected %d",
					breakEven, tt.expectBreakEven)
			}
		})
	}
}

func TestGetSupportedModels(t *testing.T) {
	calc := NewPricingCalculator()

	models := calc.GetSupportedModels()
	if len(models) == 0 {
		t.Error("Expected at least one supported model")
	}

	// Check that key models are present
	expectedModels := []string{
		"claude-3-5-sonnet-20241022",
		"claude-3-haiku-20240307",
		"claude-3-opus-20240229",
	}

	for _, expected := range expectedModels {
		found := false
		for _, model := range models {
			if model == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected model %s not found in supported models", expected)
		}
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost     float64
		expected string
	}{
		{0.0001, "$0.000100"},
		{0.001, "$0.0010"},
		{0.01, "$0.010"},
		{0.1, "$0.100"},
		{1.0, "$1.00"},
		{10.5, "$10.50"},
		{100.123, "$100.12"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			formatted := FormatCost(tt.cost)
			if formatted != tt.expected {
				t.Errorf("pricing.FormatCost(%.6f) = %s, expected %s",
					tt.cost, formatted, tt.expected)
			}
		})
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		tokens   int
		expected string
	}{
		{100, "100"},
		{999, "999"},
		{1000, "1.0K"},
		{1500, "1.5K"},
		{10000, "10.0K"},
		{999999, "1000.0K"},
		{1000000, "1.0M"},
		{1500000, "1.5M"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			formatted := FormatTokens(tt.tokens)
			if formatted != tt.expected {
				t.Errorf("pricing.FormatTokens(%d) = %s, expected %s",
					tt.tokens, formatted, tt.expected)
			}
		})
	}
}

func TestCalculateSavingsAtN(t *testing.T) {
	tests := []struct {
		name                     string
		baseCost                 float64
		firstRequestCost         float64
		subsequentRequestCost    float64
		n                        int
		expectedSavings          float64
	}{
		{
			name:                     "Positive savings",
			baseCost:                 1.0,
			firstRequestCost:        1.2,
			subsequentRequestCost:    0.1,
			n:                        10,
			expectedSavings:          7.9, // 10*1.0 - (1.2 + 9*0.1)
		},
		{
			name:                     "Break even at 2",
			baseCost:                 1.0,
			firstRequestCost:        1.5,
			subsequentRequestCost:    0.5,
			n:                        2,
			expectedSavings:          0.0, // 2*1.0 - (1.5 + 1*0.5)
		},
		{
			name:                     "Zero requests",
			baseCost:                 1.0,
			firstRequestCost:        1.5,
			subsequentRequestCost:    0.5,
			n:                        0,
			expectedSavings:          0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			savings := calculateSavingsAtN(
				tt.baseCost,
				tt.firstRequestCost,
				tt.subsequentRequestCost,
				tt.n,
			)

			if math.Abs(savings-tt.expectedSavings) > 0.00001 {
				t.Errorf("calculateSavingsAtN() = %.6f, expected %.6f",
					savings, tt.expectedSavings)
			}
		})
	}
}