package pricing

import (
	"fmt"
	"math"
	"strings"

	"autocache/internal/types"
)

// ModelPricing represents pricing information for a specific model
type ModelPricing struct {
	ModelName    string  `json:"model_name"`
	InputTokens  float64 `json:"input_tokens"`  // Price per 1M input tokens
	OutputTokens float64 `json:"output_tokens"` // Price per 1M output tokens
	CacheWrite5m float64 `json:"cache_write_5m"` // Price per 1M cache write tokens (5 minute TTL)
	CacheWrite1h float64 `json:"cache_write_1h"` // Price per 1M cache write tokens (1 hour TTL)
	CacheRead    float64 `json:"cache_read"`     // Price per 1M cache read tokens
}

// PricingCalculator handles all pricing calculations
type PricingCalculator struct {
	models map[string]ModelPricing
}

// NewPricingCalculator creates a new pricing calculator with current Anthropic pricing
func NewPricingCalculator() *PricingCalculator {
	models := map[string]ModelPricing{
		// Claude Sonnet 4.5 (Latest)
		"claude-sonnet-4-5-20250929": {
			ModelName:    "claude-sonnet-4-5-20250929",
			InputTokens:  3.00,
			OutputTokens: 15.00,
			CacheWrite5m: 3.75,  // 3.00 * 1.25
			CacheWrite1h: 6.00,  // 3.00 * 2.0
			CacheRead:    0.30,  // 3.00 * 0.1
		},

		// Claude Sonnet 4
		"claude-sonnet-4-20250514": {
			ModelName:    "claude-sonnet-4-20250514",
			InputTokens:  3.00,
			OutputTokens: 15.00,
			CacheWrite5m: 3.75,  // 3.00 * 1.25
			CacheWrite1h: 6.00,  // 3.00 * 2.0
			CacheRead:    0.30,  // 3.00 * 0.1
		},

		// Claude Sonnet 3.7 (with extended thinking)
		"claude-3-7-sonnet-20250219": {
			ModelName:    "claude-3-7-sonnet-20250219",
			InputTokens:  3.00,
			OutputTokens: 15.00,
			CacheWrite5m: 3.75,  // 3.00 * 1.25
			CacheWrite1h: 6.00,  // 3.00 * 2.0
			CacheRead:    0.30,  // 3.00 * 0.1
		},

		// Claude 3.5 Sonnet
		"claude-3-5-sonnet-20241022": {
			ModelName:    "claude-3-5-sonnet-20241022",
			InputTokens:  3.00,
			OutputTokens: 15.00,
			CacheWrite5m: 3.75,  // 3.00 * 1.25
			CacheWrite1h: 6.00,  // 3.00 * 2.0
			CacheRead:    0.30,  // 3.00 * 0.1
		},
		"claude-3-5-sonnet-20240620": {
			ModelName:    "claude-3-5-sonnet-20240620",
			InputTokens:  3.00,
			OutputTokens: 15.00,
			CacheWrite5m: 3.75,
			CacheWrite1h: 6.00,
			CacheRead:    0.30,
		},

		// Claude Opus 4.1
		"claude-opus-4-1-20250805": {
			ModelName:    "claude-opus-4-1-20250805",
			InputTokens:  15.00,
			OutputTokens: 75.00,
			CacheWrite5m: 18.75, // 15.00 * 1.25
			CacheWrite1h: 30.00, // 15.00 * 2.0
			CacheRead:    1.50,  // 15.00 * 0.1
		},

		// Claude Opus 4
		"claude-opus-4-20250514": {
			ModelName:    "claude-opus-4-20250514",
			InputTokens:  15.00,
			OutputTokens: 75.00,
			CacheWrite5m: 18.75, // 15.00 * 1.25
			CacheWrite1h: 30.00, // 15.00 * 2.0
			CacheRead:    1.50,  // 15.00 * 0.1
		},

		// Claude 3.5 Haiku
		"claude-3-5-haiku-20241022": {
			ModelName:    "claude-3-5-haiku-20241022",
			InputTokens:  0.80,
			OutputTokens: 4.00,
			CacheWrite5m: 1.00,  // 0.80 * 1.25
			CacheWrite1h: 1.60,  // 0.80 * 2.0
			CacheRead:    0.08,  // 0.80 * 0.1
		},

		// Claude 3 Opus
		"claude-3-opus-20240229": {
			ModelName:    "claude-3-opus-20240229",
			InputTokens:  15.00,
			OutputTokens: 75.00,
			CacheWrite5m: 18.75, // 15.00 * 1.25
			CacheWrite1h: 30.00, // 15.00 * 2.0
			CacheRead:    1.50,  // 15.00 * 0.1
		},

		// Claude 3 Sonnet
		"claude-3-sonnet-20240229": {
			ModelName:    "claude-3-sonnet-20240229",
			InputTokens:  3.00,
			OutputTokens: 15.00,
			CacheWrite5m: 3.75,
			CacheWrite1h: 6.00,
			CacheRead:    0.30,
		},

		// Claude 3 Haiku
		"claude-3-haiku-20240307": {
			ModelName:    "claude-3-haiku-20240307",
			InputTokens:  0.25,
			OutputTokens: 1.25,
			CacheWrite5m: 0.3125, // 0.25 * 1.25
			CacheWrite1h: 0.50,   // 0.25 * 2.0
			CacheRead:    0.025,  // 0.25 * 0.1
		},
	}

	return &PricingCalculator{
		models: models,
	}
}

// GetModelPricing returns pricing for a specific model
func (pc *PricingCalculator) GetModelPricing(model string) (ModelPricing, error) {
	if pricing, exists := pc.models[model]; exists {
		return pricing, nil
	}

	// Try to find a fuzzy match
	for modelName, pricing := range pc.models {
		if strings.Contains(model, strings.Split(modelName, "-")[0]) &&
		   strings.Contains(model, strings.Split(modelName, "-")[1]) {
			return pricing, nil
		}
	}

	// Default to Claude 3.5 Sonnet pricing if unknown
	return pc.models["claude-3-5-sonnet-20241022"], fmt.Errorf("unknown model %s, using Claude 3.5 Sonnet pricing as default", model)
}

// CalculateBaseCost calculates the cost without any caching
func (pc *PricingCalculator) CalculateBaseCost(model string, inputTokens, outputTokens int) (float64, error) {
	pricing, err := pc.GetModelPricing(model)
	if err != nil {
		return 0, err
	}

	inputCost := (float64(inputTokens) / 1_000_000) * pricing.InputTokens
	outputCost := (float64(outputTokens) / 1_000_000) * pricing.OutputTokens

	return inputCost + outputCost, nil
}

// CalculateCacheWriteCost calculates the cost of writing to cache
func (pc *PricingCalculator) CalculateCacheWriteCost(model string, tokens int, ttl string) (float64, error) {
	pricing, err := pc.GetModelPricing(model)
	if err != nil {
		return 0, err
	}

	var cachePrice float64
	switch ttl {
	case "5m":
		cachePrice = pricing.CacheWrite5m
	case "1h":
		cachePrice = pricing.CacheWrite1h
	default:
		cachePrice = pricing.CacheWrite5m // Default to 5m
	}

	return (float64(tokens) / 1_000_000) * cachePrice, nil
}

// CalculateCacheReadCost calculates the cost of reading from cache
func (pc *PricingCalculator) CalculateCacheReadCost(model string, tokens int) (float64, error) {
	pricing, err := pc.GetModelPricing(model)
	if err != nil {
		return 0, err
	}

	return (float64(tokens) / 1_000_000) * pricing.CacheRead, nil
}

// CalculateROI calculates comprehensive ROI metrics for caching decisions
func (pc *PricingCalculator) CalculateROI(model string, totalTokens, cachedTokens int, breakpoints []types.CacheBreakpoint) (types.ROIMetrics, error) {
	pricing, err := pc.GetModelPricing(model)
	if err != nil {
		return types.ROIMetrics{}, err
	}

	// Base cost without caching
	baseCost := (float64(totalTokens) / 1_000_000) * pricing.InputTokens

	// Calculate cache write costs
	totalCacheWriteCost := 0.0
	for _, bp := range breakpoints {
		writeCost, _ := pc.CalculateCacheWriteCost(model, bp.Tokens, bp.TTL)
		totalCacheWriteCost += writeCost
	}

	// Calculate cache read cost (for subsequent requests)
	cacheReadCost, _ := pc.CalculateCacheReadCost(model, cachedTokens)

	// Cost for non-cached tokens in subsequent requests
	nonCachedTokens := totalTokens - cachedTokens
	nonCachedCost := (float64(nonCachedTokens) / 1_000_000) * pricing.InputTokens

	// Total cost for first request (includes cache writes)
	firstRequestCost := totalCacheWriteCost + nonCachedCost

	// Cost for subsequent requests (cache reads + non-cached)
	subsequentRequestCost := cacheReadCost + nonCachedCost

	// Savings per subsequent request
	subsequentSavings := baseCost - subsequentRequestCost

	// Break-even calculation
	var breakEvenRequests int
	if subsequentSavings > 0 {
		extraCost := firstRequestCost - baseCost
		breakEvenRequests = int(extraCost/subsequentSavings) + 1
	} else {
		breakEvenRequests = -1 // Never breaks even
	}

	// Calculate savings at different scales
	savingsAt10 := calculateSavingsAtN(baseCost, firstRequestCost, subsequentRequestCost, 10)
	savingsAt100 := calculateSavingsAtN(baseCost, firstRequestCost, subsequentRequestCost, 100)

	// Percentage savings (at scale)
	percentSavings := 0.0
	if baseCost > 0 {
		percentSavings = (subsequentSavings / baseCost) * 100
	}

	return types.ROIMetrics{
		BaseInputCost:        baseCost,
		CacheWriteCost:       totalCacheWriteCost,
		CacheReadCost:        cacheReadCost,
		FirstRequestCost:     firstRequestCost,
		SubsequentSavings:    subsequentSavings,
		BreakEvenRequests:    breakEvenRequests,
		SavingsAt10Requests:  savingsAt10,
		SavingsAt100Requests: savingsAt100,
		PercentSavings:       percentSavings,
	}, nil
}

// calculateSavingsAtN calculates total savings after N requests
func calculateSavingsAtN(baseCost, firstRequestCost, subsequentRequestCost float64, n int) float64 {
	if n <= 0 {
		return 0
	}

	// Cost without caching
	totalCostWithoutCaching := baseCost * float64(n)

	// Cost with caching
	totalCostWithCaching := firstRequestCost + (subsequentRequestCost * float64(n-1))

	return totalCostWithoutCaching - totalCostWithCaching
}

// EstimateBreakpointROI estimates ROI for a potential cache breakpoint
func (pc *PricingCalculator) EstimateBreakpointROI(model string, tokens int, ttl string) (float64, float64, int, error) {
	// Base cost for these tokens
	baseCost, err := pc.CalculateBaseCost(model, tokens, 0)
	if err != nil {
		return 0, 0, 0, err
	}

	// Cache write cost
	writeCost, err := pc.CalculateCacheWriteCost(model, tokens, ttl)
	if err != nil {
		return 0, 0, 0, err
	}

	// Cache read cost
	readCost, err := pc.CalculateCacheReadCost(model, tokens)
	if err != nil {
		return 0, 0, 0, err
	}

	// Savings per read
	savingsPerRead := baseCost - readCost

	// Break-even calculation
	// extraCost is the additional cost paid on first request (write vs base)
	// savingsPerRead is how much we save on each subsequent request (base vs read)
	// breakEven = 1 (write) + ceil(extraCost / savingsPerRead) (reads needed)
	extraCost := writeCost - baseCost
	var breakEven int
	if savingsPerRead > 0 {
		readsNeeded := math.Ceil(extraCost / savingsPerRead)
		breakEven = 1 + int(readsNeeded)
	} else {
		breakEven = -1
	}

	return writeCost, savingsPerRead, breakEven, nil
}

// GetSupportedModels returns a list of supported models
func (pc *PricingCalculator) GetSupportedModels() []string {
	models := make([]string, 0, len(pc.models))
	for model := range pc.models {
		models = append(models, model)
	}
	return models
}

// FormatCost formats a cost value for display
func FormatCost(cost float64) string {
	if cost < 0.001 {
		return fmt.Sprintf("$%.6f", cost)
	} else if cost < 0.01 {
		return fmt.Sprintf("$%.4f", cost)
	} else if cost < 1.0 {
		return fmt.Sprintf("$%.3f", cost)
	} else {
		return fmt.Sprintf("$%.2f", cost)
	}
}

// FormatTokens formats token count for display
func FormatTokens(tokens int) string {
	if tokens < 1000 {
		return fmt.Sprintf("%d", tokens)
	} else if tokens < 1000000 {
		return fmt.Sprintf("%.1fK", float64(tokens)/1000)
	} else {
		return fmt.Sprintf("%.1fM", float64(tokens)/1000000)
	}
}