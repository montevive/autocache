package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// Config holds all configuration for the autocache proxy
type Config struct {
	// Server configuration
	Port string `json:"port"`
	Host string `json:"host"`

	// Anthropic API configuration
	AnthropicURL    string `json:"anthropic_url"`
	AnthropicAPIKey string `json:"anthropic_api_key"`

	// Cache configuration
	CacheStrategy string `json:"cache_strategy"`

	// Logging configuration
	LogLevel string `json:"log_level"`
	LogJSON  bool   `json:"log_json"`

	// Feature flags
	EnableMetrics   bool `json:"enable_metrics"`
	EnableDetailedROI bool `json:"enable_detailed_roi"`

	// Advanced configuration
	MaxCacheBreakpoints int     `json:"max_cache_breakpoints"`
	TokenMultiplier     float64 `json:"token_multiplier"`
	SavingsHistorySize  int     `json:"savings_history_size"`

	// Tokenizer configuration
	TokenizerMode          string `json:"tokenizer_mode"`           // "anthropic", "offline", "heuristic", "hybrid"
	LogTokenizerFailures   bool   `json:"log_tokenizer_failures"`   // Log tokenizer panics and fallbacks
	TokenizerPanicSamples  int    `json:"tokenizer_panic_samples"`  // Max chars to log in panic samples
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() (*Config, error) {
	// Try to load .env file (ignore error if file doesn't exist)
	_ = godotenv.Load()

	config := &Config{
		// Default values
		Port: getEnvWithDefault("PORT", "8080"),
		Host: getEnvWithDefault("HOST", "0.0.0.0"),

		AnthropicURL:    getEnvWithDefault("ANTHROPIC_API_URL", "https://api.anthropic.com"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),

		CacheStrategy: getEnvWithDefault("CACHE_STRATEGY", "moderate"),

		LogLevel: getEnvWithDefault("LOG_LEVEL", "info"),
		LogJSON:  getEnvBool("LOG_JSON", false),

		EnableMetrics:     getEnvBool("ENABLE_METRICS", true),
		EnableDetailedROI: getEnvBool("ENABLE_DETAILED_ROI", true),

		MaxCacheBreakpoints: getEnvInt("MAX_CACHE_BREAKPOINTS", 4),
		TokenMultiplier:     getEnvFloat("TOKEN_MULTIPLIER", 1.0),
		SavingsHistorySize:  getEnvInt("SAVINGS_HISTORY_SIZE", 100),

		TokenizerMode:         getEnvWithDefault("TOKENIZER_MODE", "offline"),
		LogTokenizerFailures:  getEnvBool("LOG_TOKENIZER_FAILURES", true),
		TokenizerPanicSamples: getEnvInt("TOKENIZER_PANIC_SAMPLES", 200),
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate port
	if c.Port == "" {
		return fmt.Errorf("port cannot be empty")
	}

	// Validate Anthropic URL
	if c.AnthropicURL == "" {
		return fmt.Errorf("anthropic URL cannot be empty")
	}

	// Validate cache strategy
	validStrategies := map[string]bool{
		"conservative": true,
		"moderate":     true,
		"aggressive":   true,
	}

	if !validStrategies[c.CacheStrategy] {
		return fmt.Errorf("invalid cache strategy: %s (must be one of: conservative, moderate, aggressive)", c.CacheStrategy)
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"trace": true,
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
		"fatal": true,
		"panic": true,
	}

	if !validLogLevels[c.LogLevel] {
		return fmt.Errorf("invalid log level: %s", c.LogLevel)
	}

	// Validate max cache breakpoints
	if c.MaxCacheBreakpoints < 1 || c.MaxCacheBreakpoints > 4 {
		return fmt.Errorf("max cache breakpoints must be between 1 and 4, got: %d", c.MaxCacheBreakpoints)
	}

	// Validate token multiplier
	if c.TokenMultiplier <= 0 {
		return fmt.Errorf("token multiplier must be positive, got: %f", c.TokenMultiplier)
	}

	// Validate savings history size
	if c.SavingsHistorySize < 0 {
		return fmt.Errorf("savings history size cannot be negative, got: %d", c.SavingsHistorySize)
	}

	// Validate tokenizer mode
	validTokenizerModes := map[string]bool{
		"anthropic": true,
		"offline":   true,
		"heuristic": true,
		"hybrid":    true,
	}

	if !validTokenizerModes[c.TokenizerMode] {
		return fmt.Errorf("invalid tokenizer mode: %s (must be one of: anthropic, offline, heuristic, hybrid)", c.TokenizerMode)
	}

	// Validate tokenizer panic samples
	if c.TokenizerPanicSamples < 0 {
		return fmt.Errorf("tokenizer panic samples cannot be negative, got: %d", c.TokenizerPanicSamples)
	}

	return nil
}

// GetServerAddress returns the full server address
func (c *Config) GetServerAddress() string {
	return c.Host + ":" + c.Port
}

// GetLogLevel returns the logrus log level
func (c *Config) GetLogLevel() logrus.Level {
	level, err := logrus.ParseLevel(c.LogLevel)
	if err != nil {
		return logrus.InfoLevel // Default fallback
	}
	return level
}

// IsAPIKeyConfigured checks if an API key is configured
func (c *Config) IsAPIKeyConfigured() bool {
	return strings.TrimSpace(c.AnthropicAPIKey) != ""
}

// PrintConfig prints the configuration (redacts sensitive information)
func (c *Config) PrintConfig(logger *logrus.Logger) {
	apiKey := "not configured"
	if c.IsAPIKeyConfigured() {
		apiKey = "configured (***redacted***)"
	}

	logger.WithFields(logrus.Fields{
		"server_address":         c.GetServerAddress(),
		"anthropic_url":          c.AnthropicURL,
		"anthropic_api_key":      apiKey,
		"cache_strategy":         c.CacheStrategy,
		"log_level":              c.LogLevel,
		"log_json":               c.LogJSON,
		"enable_metrics":         c.EnableMetrics,
		"enable_detailed_roi":    c.EnableDetailedROI,
		"max_cache_breakpoints":  c.MaxCacheBreakpoints,
		"token_multiplier":       c.TokenMultiplier,
		"savings_history_size":   c.SavingsHistorySize,
		"tokenizer_mode":         c.TokenizerMode,
		"log_tokenizer_failures": c.LogTokenizerFailures,
	}).Info("Configuration loaded")
}

// SetupLogger configures and returns a logger based on config
func (c *Config) SetupLogger() *logrus.Logger {
	logger := logrus.New()

	// Set log level
	logger.SetLevel(c.GetLogLevel())

	// Set log format
	if c.LogJSON {
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
		})
	} else {
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}

	return logger
}

// GetEnvironmentInfo returns information about the environment
func GetEnvironmentInfo() map[string]string {
	return map[string]string{
		"GO_VERSION": os.Getenv("GO_VERSION"),
		"BUILD_TIME": os.Getenv("BUILD_TIME"),
		"GIT_COMMIT": os.Getenv("GIT_COMMIT"),
		"VERSION":    os.Getenv("VERSION"),
	}
}

// Helper functions for environment variable parsing

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch strings.ToLower(value) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}

// ConfigSummary returns a summary of the current configuration for API responses
func (c *Config) ConfigSummary() map[string]interface{} {
	return map[string]interface{}{
		"cache_strategy":        c.CacheStrategy,
		"max_cache_breakpoints": c.MaxCacheBreakpoints,
		"token_multiplier":      c.TokenMultiplier,
		"api_key_configured":    c.IsAPIKeyConfigured(),
		"metrics_enabled":       c.EnableMetrics,
		"detailed_roi_enabled":  c.EnableDetailedROI,
	}
}

// UpdateFromEnvironment updates configuration from current environment variables
// This can be useful for runtime configuration updates
func (c *Config) UpdateFromEnvironment() error {
	newConfig, err := LoadConfig()
	if err != nil {
		return err
	}

	// Update non-sensitive fields that can be changed at runtime
	c.CacheStrategy = newConfig.CacheStrategy
	c.LogLevel = newConfig.LogLevel
	c.LogJSON = newConfig.LogJSON
	c.EnableMetrics = newConfig.EnableMetrics
	c.EnableDetailedROI = newConfig.EnableDetailedROI
	c.MaxCacheBreakpoints = newConfig.MaxCacheBreakpoints
	c.TokenMultiplier = newConfig.TokenMultiplier

	return c.Validate()
}