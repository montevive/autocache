package cache

import (
	"fmt"
	"time"

	"autocache/internal/config"
	"autocache/internal/pricing"
	"autocache/internal/tokenizer"
	"autocache/internal/types"

	"github.com/sirupsen/logrus"
)

// CacheInjector handles intelligent cache control injection
type CacheInjector struct {
	tokenizer tokenizer.Tokenizer
	pricing   *pricing.PricingCalculator
	strategy  types.CacheStrategy
	logger    *logrus.Logger
}

// NewCacheInjector creates a new cache injector
func NewCacheInjector(strategy types.CacheStrategy, anthropicURL, apiKey string, logger *logrus.Logger) *CacheInjector {
	// Use heuristic tokenizer for simplicity and reliability
	// For production use, consider NewCacheInjectorWithConfig for more accurate tokenizers
	tk := tokenizer.NewAnthropicTokenizer()

	return &CacheInjector{
		tokenizer: tk,
		pricing:   pricing.NewPricingCalculator(),
		strategy:  strategy,
		logger:    logger,
	}
}

// NewCacheInjectorWithConfig creates a new cache injector with config-based tokenizer selection
func NewCacheInjectorWithConfig(strategy types.CacheStrategy, cfg *config.Config, logger *logrus.Logger) *CacheInjector {
	var tk tokenizer.Tokenizer
	var err error

	// Select tokenizer based on configuration
	switch cfg.TokenizerMode {
	case "anthropic":
		// Use official Anthropic tokenizer for highest accuracy
		logger.Info("Initializing Anthropic real tokenizer (this may take a moment...)")
		tk, err = tokenizer.NewAnthropicRealTokenizerWithLogger(logger)
		if err != nil {
			logger.WithError(err).Fatal("Failed to initialize Anthropic tokenizer")
		}
		logger.Info("Anthropic tokenizer ready - accurate token counting enabled")

	case "offline":
		// Use offline tokenizer with panic recovery
		logger.Info("Initializing offline tokenizer with panic recovery")
		tk, err = tokenizer.NewOfflineTokenizerWithLogger(logger)
		if err != nil {
			logger.WithError(err).Fatal("Failed to initialize offline tokenizer")
		}

	case "heuristic":
		// Use fast heuristic tokenizer (approximation)
		logger.Info("Using heuristic tokenizer (fast approximation)")
		tk = tokenizer.NewAnthropicTokenizer()

	case "hybrid":
		// Use offline with heuristic fallback (default behavior)
		logger.Info("Using hybrid tokenizer (offline with heuristic fallback)")
		tk, err = tokenizer.NewOfflineTokenizerWithLogger(logger)
		if err != nil {
			logger.WithError(err).Warn("Failed to initialize offline tokenizer, falling back to heuristic")
			tk = tokenizer.NewAnthropicTokenizer()
		}

	default:
		logger.WithField("mode", cfg.TokenizerMode).Warn("Unknown tokenizer mode, using heuristic")
		tk = tokenizer.NewAnthropicTokenizer()
	}

	return &CacheInjector{
		tokenizer: tk,
		pricing:   pricing.NewPricingCalculator(),
		strategy:  strategy,
		logger:    logger,
	}
}

// GetTokenizer returns the tokenizer instance (for access to methods like GetPanicStats)
func (ci *CacheInjector) GetTokenizer() tokenizer.Tokenizer {
	return ci.tokenizer
}

// GetPricing returns the pricing calculator instance
func (ci *CacheInjector) GetPricing() *pricing.PricingCalculator {
	return ci.pricing
}

// CacheCandidate represents a potential cache breakpoint
type CacheCandidate struct {
	Position     string  // "system", "tools", "message_0_block_1", etc.
	Tokens       int     // Token count
	ContentType  string  // "system", "tools", "content"
	TTL          string  // "5m" or "1h"
	ROIScore     float64 // ROI score for prioritization
	WriteCost    float64 // Cost to write cache
	ReadSavings  float64 // Savings per read
	BreakEven    int     // Requests to break even
	Content      interface{} // Reference to the actual content (for modification)
}

// InjectCacheControl analyzes a request and injects optimal cache control
func (ci *CacheInjector) InjectCacheControl(req *types.AnthropicRequest) (*types.CacheMetadata, error) {
	startTime := time.Now()

	ci.logger.WithFields(logrus.Fields{
		"model":    req.Model,
		"strategy": ci.strategy,
	}).Debug("Starting cache injection analysis")

	// Get strategy configuration
	strategyConfig := types.GetStrategyConfig(ci.strategy)
	minimumTokens := ci.tokenizer.GetModelMinimumTokens(req.Model)
	adjustedMinimum := int(float64(minimumTokens) * strategyConfig.MinTokensMultiplier)

	// Collect all cache candidates in deterministic order (system → tools → messages)
	candidates := ci.CollectCacheCandidates(req, adjustedMinimum, strategyConfig)

	// Candidates are already in deterministic order, no sorting needed
	// This ensures consistent breakpoint placement: system → tools → messages

	// Select top candidates respecting breakpoint limit
	maxBreakpoints := strategyConfig.MaxBreakpoints
	if len(candidates) > maxBreakpoints {
		candidates = candidates[:maxBreakpoints]
	}

	// Apply cache control to selected candidates
	breakpoints := ci.ApplyCacheControl(candidates)

	// Calculate metadata
	metadata := ci.calculateMetadata(req, breakpoints, startTime)

	ci.logger.WithFields(logrus.Fields{
		"total_tokens":   metadata.TotalTokens,
		"cached_tokens":  metadata.CachedTokens,
		"cache_ratio":    metadata.CacheRatio,
		"breakpoints":    len(breakpoints),
		"roi_percent":    metadata.ROI.PercentSavings,
		"break_even":     metadata.ROI.BreakEvenRequests,
	}).Info("Cache injection completed")

	return metadata, nil
}

// CollectCacheCandidates finds all potential cache breakpoints
func (ci *CacheInjector) CollectCacheCandidates(req *types.AnthropicRequest, minTokens int, strategyConfig types.StrategyConfig) []CacheCandidate {
	var candidates []CacheCandidate

	// Check system content
	if req.System != "" {
		tokens := ci.tokenizer.CountSystemTokens(req.System)
		if tokens >= minTokens {
			candidate := ci.CreateCandidate("system", tokens, "system", strategyConfig.SystemTTL, req.Model, &req.System)
			candidates = append(candidates, candidate)
		}
	}

	// Check system blocks
	if len(req.SystemBlocks) > 0 {
		tokens := ci.tokenizer.CountSystemBlocksTokens(req.SystemBlocks)
		if tokens >= minTokens {
			candidate := ci.CreateCandidate("system_blocks", tokens, "system", strategyConfig.SystemTTL, req.Model, &req.SystemBlocks)
			candidates = append(candidates, candidate)
		}
	}

	// Check tools
	if len(req.Tools) > 0 {
		totalToolTokens := 0
		for _, tool := range req.Tools {
			totalToolTokens += ci.tokenizer.CountToolTokens(tool)
		}
		if totalToolTokens >= minTokens {
			candidate := ci.CreateCandidate("tools", totalToolTokens, "tools", strategyConfig.ToolsTTL, req.Model, &req.Tools)
			candidates = append(candidates, candidate)
		}
	}

	// Check message content blocks
	for msgIdx, message := range req.Messages {
		for blockIdx, block := range message.Content {
			if block.Type == "text" && block.Text != "" {
				tokens := ci.tokenizer.CountTokens(block.Text)
				if tokens >= minTokens {
					position := fmt.Sprintf("message_%d_block_%d", msgIdx, blockIdx)

					// Determine TTL based on content characteristics
					ttl := ci.DetermineTTLForContent(block.Text, strategyConfig)

					candidate := ci.CreateCandidate(position, tokens, "content", ttl, req.Model, &req.Messages[msgIdx].Content[blockIdx])
					candidates = append(candidates, candidate)
				}
			}
		}
	}

	return candidates
}

// CreateCandidate creates a cache candidate with ROI calculation
func (ci *CacheInjector) CreateCandidate(position string, tokens int, contentType, ttl, model string, content interface{}) CacheCandidate {
	writeCost, readSavings, breakEven, _ := ci.pricing.EstimateBreakpointROI(model, tokens, ttl)

	// Calculate ROI score for prioritization
	roiScore := ci.CalculateROIScore(tokens, writeCost, readSavings, breakEven, contentType)

	return CacheCandidate{
		Position:    position,
		Tokens:      tokens,
		ContentType: contentType,
		TTL:         ttl,
		ROIScore:    roiScore,
		WriteCost:   writeCost,
		ReadSavings: readSavings,
		BreakEven:   breakEven,
		Content:     content,
	}
}

// CalculateROIScore calculates a score for prioritizing cache candidates
func (ci *CacheInjector) CalculateROIScore(tokens int, writeCost, readSavings float64, breakEven int, contentType string) float64 {
	// Base score from savings potential
	score := readSavings * 100 // Scale up for easier comparison

	// Bonus for larger content (more likely to be reused)
	if tokens > 2048 {
		score *= 1.2
	}
	if tokens > 5000 {
		score *= 1.3
	}

	// Content type bonuses (stable content is preferred)
	switch contentType {
	case "system":
		score *= 2.0 // Highest priority - very stable
	case "tools":
		score *= 1.5 // High priority - fairly stable
	case "content":
		// Content score depends on break-even point
		if breakEven <= 2 {
			score *= 1.3 // Quick break-even
		} else if breakEven <= 5 {
			score *= 1.1 // Reasonable break-even
		}
		// No bonus for high break-even content
	}

	// Penalty for very high break-even points
	if breakEven > 10 {
		score *= 0.5
	} else if breakEven > 20 {
		score *= 0.2
	}

	return score
}

// DetermineTTLForContent determines appropriate TTL based on content characteristics
func (ci *CacheInjector) DetermineTTLForContent(text string, strategyConfig types.StrategyConfig) string {
	// Check for stable patterns that might benefit from longer caching
	stablePatterns := []string{
		"You are", "Your role", "Instructions:", "Guidelines:",
		"System:", "Context:", "Background:", "Reference:",
	}

	for _, pattern := range stablePatterns {
		if len(text) > 1000 && containsCaseInsensitive(text, pattern) {
			return "1h" // More stable content gets longer TTL
		}
	}

	// Default to content TTL from strategy
	return strategyConfig.ContentTTL
}

// ApplyCacheControl applies cache control to the selected candidates
func (ci *CacheInjector) ApplyCacheControl(candidates []CacheCandidate) []types.CacheBreakpoint {
	var breakpoints []types.CacheBreakpoint

	for _, candidate := range candidates {
		// Create cache control object
		cacheControl := &types.CacheControl{
			Type: "ephemeral",
			TTL:  candidate.TTL,
		}

		// Apply cache control based on content type
		applied := ci.applyCacheControlToContent(candidate.Content, cacheControl)

		if applied {
			breakpoint := types.CacheBreakpoint{
				Position:    candidate.Position,
				Tokens:      candidate.Tokens,
				TTL:         candidate.TTL,
				Type:        candidate.ContentType,
				WritePrice:  candidate.WriteCost,
				ReadSavings: candidate.ReadSavings,
				Timestamp:   time.Now(),
			}
			breakpoints = append(breakpoints, breakpoint)

			ci.logger.WithFields(logrus.Fields{
				"position":      candidate.Position,
				"tokens":        candidate.Tokens,
				"ttl":           candidate.TTL,
				"write_cost":    pricing.FormatCost(candidate.WriteCost),
				"read_savings":  pricing.FormatCost(candidate.ReadSavings),
				"break_even":    candidate.BreakEven,
			}).Debug("Applied cache control")
		}
	}

	return breakpoints
}

// applyCacheControlToContent applies cache control to the actual content structures
func (ci *CacheInjector) applyCacheControlToContent(content interface{}, cacheControl *types.CacheControl) bool {
	switch v := content.(type) {
	case *string:
		// For system strings, we can't directly add cache control
		// This would need to be handled at the request level
		return true

	case *[]types.ContentBlock:
		// Add cache control to the last block
		if len(*v) > 0 {
			(*v)[len(*v)-1].CacheControl = cacheControl
			return true
		}

	case *[]types.ToolDefinition:
		// Add cache control to the last tool
		if len(*v) > 0 {
			(*v)[len(*v)-1].CacheControl = cacheControl
			return true
		}

	case *types.ContentBlock:
		// Direct content block
		v.CacheControl = cacheControl
		return true
	}

	return false
}

// calculateMetadata calculates comprehensive metadata about the caching decisions
func (ci *CacheInjector) calculateMetadata(req *types.AnthropicRequest, breakpoints []types.CacheBreakpoint, startTime time.Time) *types.CacheMetadata {
	// Calculate total tokens
	totalTokens := ci.tokenizer.EstimateRequestTokens(req)

	// Calculate cached tokens
	cachedTokens := 0
	for _, bp := range breakpoints {
		cachedTokens += bp.Tokens
	}

	// Calculate cache ratio
	cacheRatio := 0.0
	if totalTokens > 0 {
		cacheRatio = float64(cachedTokens) / float64(totalTokens)
	}

	// Calculate ROI
	roi, _ := ci.pricing.CalculateROI(req.Model, totalTokens, cachedTokens, breakpoints)

	return &types.CacheMetadata{
		CacheInjected: len(breakpoints) > 0,
		TotalTokens:   totalTokens,
		CachedTokens:  cachedTokens,
		CacheRatio:    cacheRatio,
		Breakpoints:   breakpoints,
		ROI:           roi,
		Strategy:      string(ci.strategy),
		Model:         req.Model,
		Timestamp:     time.Now(),
	}
}

// Helper function for case-insensitive string contains
func containsCaseInsensitive(text, substr string) bool {
	return len(text) >= len(substr) &&
		   len(substr) > 0 &&
		   findSubstringIgnoreCase(text, substr)
}

func findSubstringIgnoreCase(text, substr string) bool {
	textLower := make([]rune, 0, len(text))
	substrLower := make([]rune, 0, len(substr))

	for _, r := range text {
		if r >= 'A' && r <= 'Z' {
			textLower = append(textLower, r+32)
		} else {
			textLower = append(textLower, r)
		}
	}

	for _, r := range substr {
		if r >= 'A' && r <= 'Z' {
			substrLower = append(substrLower, r+32)
		} else {
			substrLower = append(substrLower, r)
		}
	}

	textStr := string(textLower)
	substrStr := string(substrLower)

	for i := 0; i <= len(textStr)-len(substrStr); i++ {
		if textStr[i:i+len(substrStr)] == substrStr {
			return true
		}
	}
	return false
}