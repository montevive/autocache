package types

import (
	"encoding/json"
	"fmt"
	"time"
)

// CacheControl represents the cache control configuration
type CacheControl struct {
	Type string `json:"type"` // "ephemeral"
	TTL  string `json:"ttl"`  // "5m" or "1h"
}

// ContentBlock represents a content block in a message
type ContentBlock struct {
	Type         string        `json:"type"`
	Text         string        `json:"text,omitempty"`
	Source       *ImageSource  `json:"source,omitempty"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`

	// For tool_use blocks (assistant messages)
	ID    string      `json:"id,omitempty"`
	Name  string      `json:"name,omitempty"`
	Input interface{} `json:"input,omitempty"`

	// For tool_result blocks (user messages)
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"`
	IsError   *bool       `json:"is_error,omitempty"`
}

// ImageSource represents an image source
type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// Message represents a message in the conversation
type Message struct {
	Role    string         `json:"role"`
	Content []ContentBlock `json:"content"`
}

// UnmarshalJSON implements custom unmarshaling for Message to support both string and array content formats
func (m *Message) UnmarshalJSON(data []byte) error {
	// Create a temporary struct with Content as json.RawMessage to inspect it first
	type Alias Message
	aux := &struct {
		Content json.RawMessage `json:"content"`
		*Alias
	}{
		Alias: (*Alias)(m),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// Check if content is a string (shorthand format)
	var contentStr string
	if err := json.Unmarshal(aux.Content, &contentStr); err == nil {
		// It's a string, convert to ContentBlock array
		m.Content = []ContentBlock{
			{
				Type: "text",
				Text: contentStr,
			},
		}
		return nil
	}

	// It's an array, unmarshal normally
	var contentBlocks []ContentBlock
	if err := json.Unmarshal(aux.Content, &contentBlocks); err != nil {
		return err
	}
	m.Content = contentBlocks

	return nil
}

// ToolDefinition represents a tool definition
type ToolDefinition struct {
	Name         string        `json:"name"`
	Description  string        `json:"description"`
	InputSchema  interface{}   `json:"input_schema"`
	CacheControl *CacheControl `json:"cache_control,omitempty"`
}

// AnthropicRequest represents the complete request to Anthropic API
type AnthropicRequest struct {
	Model         string           `json:"model"`
	MaxTokens     int              `json:"max_tokens"`
	Messages      []Message        `json:"messages"`
	System        string           `json:"system,omitempty"`
	SystemBlocks  []ContentBlock   `json:"-"` // Handle system blocks through custom parsing
	Tools         []ToolDefinition `json:"tools,omitempty"`
	Temperature   *float64         `json:"temperature,omitempty"`
	TopP          *float64         `json:"top_p,omitempty"`
	TopK          *int             `json:"top_k,omitempty"`
	Stream        *bool            `json:"stream,omitempty"`
	StopSequences []string         `json:"stop_sequences,omitempty"`
}

// MarshalJSON implements custom marshaling for AnthropicRequest
// When SystemBlocks is populated, it serializes as the "system" field instead of the System string
func (r *AnthropicRequest) MarshalJSON() ([]byte, error) {
	// Create an alias type to avoid infinite recursion
	type Alias AnthropicRequest

	// If SystemBlocks is populated, we need to serialize it as "system" array
	if len(r.SystemBlocks) > 0 {
		// Create an anonymous struct with all fields explicitly set
		return json.Marshal(&struct {
			Model         string           `json:"model"`
			MaxTokens     int              `json:"max_tokens"`
			Messages      []Message        `json:"messages"`
			System        interface{}      `json:"system,omitempty"`        // Use interface{} to allow array
			Tools         []ToolDefinition `json:"tools,omitempty"`
			Temperature   *float64         `json:"temperature,omitempty"`
			TopP          *float64         `json:"top_p,omitempty"`
			TopK          *int             `json:"top_k,omitempty"`
			Stream        *bool            `json:"stream,omitempty"`
			StopSequences []string         `json:"stop_sequences,omitempty"`
		}{
			Model:         r.Model,
			MaxTokens:     r.MaxTokens,
			Messages:      r.Messages,
			System:        r.SystemBlocks, // Serialize SystemBlocks as "system"
			Tools:         r.Tools,
			Temperature:   r.Temperature,
			TopP:          r.TopP,
			TopK:          r.TopK,
			Stream:        r.Stream,
			StopSequences: r.StopSequences,
		})
	}

	// Otherwise, use normal marshaling (System string field)
	return json.Marshal((*Alias)(r))
}

// UnmarshalJSON implements custom unmarshaling for AnthropicRequest
// Handles both string and array formats for the "system" field
func (r *AnthropicRequest) UnmarshalJSON(data []byte) error {
	// Create an alias type to avoid infinite recursion
	type Alias AnthropicRequest

	// First, try to unmarshal with a temporary struct that has system as RawMessage
	aux := &struct {
		*Alias
		System json.RawMessage `json:"system,omitempty"`
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// If there's a system field, check if it's a string or array
	if len(aux.System) > 0 {
		// Try to unmarshal as string first
		var systemStr string
		if err := json.Unmarshal(aux.System, &systemStr); err == nil {
			r.System = systemStr
			return nil
		}

		// If that fails, try to unmarshal as array of content blocks
		var systemBlocks []ContentBlock
		if err := json.Unmarshal(aux.System, &systemBlocks); err == nil {
			r.SystemBlocks = systemBlocks
			r.System = "" // Clear the string field
			return nil
		}

		// If both fail, return error
		return fmt.Errorf("system field must be either a string or an array of content blocks")
	}

	return nil
}

// AnthropicResponse represents the response from Anthropic API
type AnthropicResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []ContentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        Usage          `json:"usage"`
}

// Usage represents token usage information
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

// CacheBreakpoint represents a cache breakpoint decision
type CacheBreakpoint struct {
	Position    string    `json:"position"`     // "system", "tools", "message_0_block_1"
	Tokens      int       `json:"tokens"`       // Number of tokens cached
	TTL         string    `json:"ttl"`          // "5m" or "1h"
	Type        string    `json:"type"`         // "system", "tools", "content"
	WritePrice  float64   `json:"write_price"`  // Cost to write this cache
	ReadSavings float64   `json:"read_savings"` // Savings per read
	Timestamp   time.Time `json:"timestamp"`
}

// ROIMetrics represents return on investment calculations
type ROIMetrics struct {
	BaseInputCost        float64 `json:"base_input_cost"`         // Original cost without caching
	CacheWriteCost       float64 `json:"cache_write_cost"`        // Additional cost for cache writes
	CacheReadCost        float64 `json:"cache_read_cost"`         // Cost for cache reads (subsequent requests)
	FirstRequestCost     float64 `json:"first_request_cost"`      // Total cost including cache writes
	SubsequentSavings    float64 `json:"subsequent_savings"`      // Savings per subsequent request
	BreakEvenRequests    int     `json:"break_even_requests"`     // Number of requests to break even
	SavingsAt10Requests  float64 `json:"savings_at_10_requests"`  // Total savings after 10 requests
	SavingsAt100Requests float64 `json:"savings_at_100_requests"` // Total savings after 100 requests
	PercentSavings       float64 `json:"percent_savings"`         // Percentage savings at scale
}

// CacheMetadata represents metadata about caching decisions
type CacheMetadata struct {
	CacheInjected bool               `json:"cache_injected"`
	TotalTokens   int                `json:"total_tokens"`
	CachedTokens  int                `json:"cached_tokens"`
	CacheRatio    float64            `json:"cache_ratio"` // Percentage of tokens cached
	Breakpoints   []CacheBreakpoint  `json:"breakpoints"`
	ROI           ROIMetrics         `json:"roi"`
	Strategy      string             `json:"strategy"` // "aggressive", "moderate", "conservative"
	Model         string             `json:"model"`
	Timestamp     time.Time          `json:"timestamp"`
}

// CacheStrategy represents different caching strategies
type CacheStrategy string

const (
	StrategyConservative CacheStrategy = "conservative"
	StrategyModerate     CacheStrategy = "moderate"
	StrategyAggressive   CacheStrategy = "aggressive"
)

// StrategyConfig represents configuration for each strategy
type StrategyConfig struct {
	MaxBreakpoints      int      `json:"max_breakpoints"`
	MinTokensMultiplier float64  `json:"min_tokens_multiplier"` // Multiplier for base minimum tokens
	SystemTTL           string   `json:"system_ttl"`
	ToolsTTL            string   `json:"tools_ttl"`
	ContentTTL          string   `json:"content_ttl"`
	Priority            []string `json:"priority"` // Order of content types to prioritize
}

// GetStrategyConfig returns configuration for a given strategy
func GetStrategyConfig(strategy CacheStrategy) StrategyConfig {
	configs := map[CacheStrategy]StrategyConfig{
		StrategyConservative: {
			MaxBreakpoints:      2,
			MinTokensMultiplier: 2.0, // More strict token requirements
			SystemTTL:           "1h",
			ToolsTTL:            "1h",
			ContentTTL:          "5m",
			Priority:            []string{"system", "tools"},
		},
		StrategyModerate: {
			MaxBreakpoints:      3,
			MinTokensMultiplier: 1.0, // Standard token requirements
			SystemTTL:           "1h",
			ToolsTTL:            "1h",
			ContentTTL:          "5m",
			Priority:            []string{"system", "tools", "content"},
		},
		StrategyAggressive: {
			MaxBreakpoints:      4,
			MinTokensMultiplier: 0.8, // More lenient token requirements
			SystemTTL:           "1h",
			ToolsTTL:            "1h",
			ContentTTL:          "5m",
			Priority:            []string{"system", "tools", "content", "large_content"},
		},
	}
	return configs[strategy]
}

// ToHeaderValue converts a struct to a compact string for headers
func (cm *CacheMetadata) ToHeaderValue() string {
	data, _ := json.Marshal(cm)
	return string(data)
}

// ToBreakpointsHeader converts breakpoints to a compact header string
func (cm *CacheMetadata) ToBreakpointsHeader() string {
	if len(cm.Breakpoints) == 0 {
		return ""
	}

	result := ""
	for i, bp := range cm.Breakpoints {
		if i > 0 {
			result += ","
		}
		result += bp.Position + ":" + string(rune(bp.Tokens)) + ":" + bp.TTL
	}
	return result
}
