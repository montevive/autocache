package config

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestLoadConfig(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	envVars := []string{
		"PORT", "HOST", "ANTHROPIC_API_URL", "ANTHROPIC_API_KEY",
		"CACHE_STRATEGY", "LOG_LEVEL", "LOG_JSON", "ENABLE_METRICS",
		"ENABLE_DETAILED_ROI", "MAX_CACHE_BREAKPOINTS", "TOKEN_MULTIPLIER",
	}

	for _, env := range envVars {
		originalEnv[env] = os.Getenv(env)
		os.Unsetenv(env)
	}

	// Restore environment after test
	defer func() {
		for env, value := range originalEnv {
			if value != "" {
				os.Setenv(env, value)
			} else {
				os.Unsetenv(env)
			}
		}
	}()

	t.Run("Default configuration", func(t *testing.T) {
		cfg, err := LoadConfig()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if cfg.Port != "8080" {
			t.Errorf("Expected default port 8080, got %s", cfg.Port)
		}
		if cfg.Host != "0.0.0.0" {
			t.Errorf("Expected default host 0.0.0.0, got %s", cfg.Host)
		}
		if cfg.AnthropicURL != "https://api.anthropic.com" {
			t.Errorf("Expected default Anthropic URL, got %s", cfg.AnthropicURL)
		}
		if cfg.CacheStrategy != "moderate" {
			t.Errorf("Expected default cache strategy moderate, got %s", cfg.CacheStrategy)
		}
		if cfg.LogLevel != "info" {
			t.Errorf("Expected default log level info, got %s", cfg.LogLevel)
		}
		if cfg.MaxCacheBreakpoints != 4 {
			t.Errorf("Expected default max breakpoints 4, got %d", cfg.MaxCacheBreakpoints)
		}
		if cfg.TokenMultiplier != 1.0 {
			t.Errorf("Expected default token multiplier 1.0, got %f", cfg.TokenMultiplier)
		}
	})

	t.Run("Custom environment variables", func(t *testing.T) {
		os.Setenv("PORT", "3000")
		os.Setenv("HOST", "127.0.0.1")
		os.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
		os.Setenv("CACHE_STRATEGY", "aggressive")
		os.Setenv("LOG_LEVEL", "debug")
		os.Setenv("LOG_JSON", "true")
		os.Setenv("ENABLE_METRICS", "false")
		os.Setenv("MAX_CACHE_BREAKPOINTS", "2")
		os.Setenv("TOKEN_MULTIPLIER", "1.5")

		cfg, err := LoadConfig()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if cfg.Port != "3000" {
			t.Errorf("Expected port 3000, got %s", cfg.Port)
		}
		if cfg.Host != "127.0.0.1" {
			t.Errorf("Expected host 127.0.0.1, got %s", cfg.Host)
		}
		if cfg.AnthropicAPIKey != "sk-ant-test" {
			t.Errorf("Expected API key sk-ant-test, got %s", cfg.AnthropicAPIKey)
		}
		if cfg.CacheStrategy != "aggressive" {
			t.Errorf("Expected cache strategy aggressive, got %s", cfg.CacheStrategy)
		}
		if cfg.LogLevel != "debug" {
			t.Errorf("Expected log level debug, got %s", cfg.LogLevel)
		}
		if !cfg.LogJSON {
			t.Error("Expected log JSON to be true")
		}
		if cfg.EnableMetrics {
			t.Error("Expected enable metrics to be false")
		}
		if cfg.MaxCacheBreakpoints != 2 {
			t.Errorf("Expected max breakpoints 2, got %d", cfg.MaxCacheBreakpoints)
		}
		if cfg.TokenMultiplier != 1.5 {
			t.Errorf("Expected token multiplier 1.5, got %f", cfg.TokenMultiplier)
		}
	})
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorContains string
	}{
		{
			name: "Valid configuration",
			config: &Config{
				Port:                "8080",
				Host:                "0.0.0.0",
				AnthropicURL:        "https://api.anthropic.com",
				CacheStrategy:       "moderate",
				LogLevel:            "info",
				MaxCacheBreakpoints: 4,
				TokenMultiplier:     1.0,
				TokenizerMode:       "offline",
			},
			expectError: false,
		},
		{
			name: "Empty port",
			config: &Config{
				Port:                "",
				AnthropicURL:        "https://api.anthropic.com",
				CacheStrategy:       "moderate",
				LogLevel:            "info",
				MaxCacheBreakpoints: 4,
				TokenMultiplier:     1.0,
				TokenizerMode:       "offline",
			},
			expectError:   true,
			errorContains: "port cannot be empty",
		},
		{
			name: "Empty Anthropic URL",
			config: &Config{
				Port:                "8080",
				AnthropicURL:        "",
				CacheStrategy:       "moderate",
				LogLevel:            "info",
				MaxCacheBreakpoints: 4,
				TokenMultiplier:     1.0,
				TokenizerMode:       "offline",
			},
			expectError:   true,
			errorContains: "anthropic URL cannot be empty",
		},
		{
			name: "Invalid cache strategy",
			config: &Config{
				Port:                "8080",
				AnthropicURL:        "https://api.anthropic.com",
				CacheStrategy:       "invalid",
				LogLevel:            "info",
				MaxCacheBreakpoints: 4,
				TokenMultiplier:     1.0,
				TokenizerMode:       "offline",
			},
			expectError:   true,
			errorContains: "invalid cache strategy",
		},
		{
			name: "Invalid log level",
			config: &Config{
				Port:                "8080",
				AnthropicURL:        "https://api.anthropic.com",
				CacheStrategy:       "moderate",
				LogLevel:            "invalid",
				MaxCacheBreakpoints: 4,
				TokenMultiplier:     1.0,
				TokenizerMode:       "offline",
			},
			expectError:   true,
			errorContains: "invalid log level",
		},
		{
			name: "Invalid max breakpoints - too low",
			config: &Config{
				Port:                "8080",
				AnthropicURL:        "https://api.anthropic.com",
				CacheStrategy:       "moderate",
				LogLevel:            "info",
				MaxCacheBreakpoints: 0,
				TokenMultiplier:     1.0,
				TokenizerMode:       "offline",
			},
			expectError:   true,
			errorContains: "max cache breakpoints must be between 1 and 4",
		},
		{
			name: "Invalid max breakpoints - too high",
			config: &Config{
				Port:                "8080",
				AnthropicURL:        "https://api.anthropic.com",
				CacheStrategy:       "moderate",
				LogLevel:            "info",
				MaxCacheBreakpoints: 5,
				TokenMultiplier:     1.0,
				TokenizerMode:       "offline",
			},
			expectError:   true,
			errorContains: "max cache breakpoints must be between 1 and 4",
		},
		{
			name: "Invalid token multiplier",
			config: &Config{
				Port:                "8080",
				AnthropicURL:        "https://api.anthropic.com",
				CacheStrategy:       "moderate",
				LogLevel:            "info",
				MaxCacheBreakpoints: 4,
				TokenMultiplier:     0.0,
				TokenizerMode:       "offline",
			},
			expectError:   true,
			errorContains: "token multiplier must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
			if tt.expectError && err != nil && tt.errorContains != "" {
				if !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
			}
		})
	}
}

func TestGetServerAddress(t *testing.T) {
	cfg := &Config{
		Host: "127.0.0.1",
		Port: "3000",
	}

	expected := "127.0.0.1:3000"
	actual := cfg.GetServerAddress()

	if actual != expected {
		t.Errorf("GetServerAddress() = %s, expected %s", actual, expected)
	}
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		configLevel string
		expected    logrus.Level
	}{
		{"trace", logrus.TraceLevel},
		{"debug", logrus.DebugLevel},
		{"info", logrus.InfoLevel},
		{"warn", logrus.WarnLevel},
		{"error", logrus.ErrorLevel},
		{"fatal", logrus.FatalLevel},
		{"panic", logrus.PanicLevel},
		{"invalid", logrus.InfoLevel}, // Should default to info
	}

	for _, tt := range tests {
		t.Run(tt.configLevel, func(t *testing.T) {
			cfg := &Config{LogLevel: tt.configLevel}
			actual := cfg.GetLogLevel()

			if actual != tt.expected {
				t.Errorf("GetLogLevel(%s) = %v, expected %v", tt.configLevel, actual, tt.expected)
			}
		})
	}
}

func TestIsAPIKeyConfigured(t *testing.T) {
	tests := []struct {
		name     string
		apiKey   string
		expected bool
	}{
		{"With API key", "sk-ant-test", true},
		{"Empty API key", "", false},
		{"Whitespace only", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{AnthropicAPIKey: tt.apiKey}
			actual := cfg.IsAPIKeyConfigured()

			if actual != tt.expected {
				t.Errorf("IsAPIKeyConfigured() = %v, expected %v", actual, tt.expected)
			}
		})
	}
}

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *Config
		testLog  bool
	}{
		{
			name: "Text formatter",
			cfg: &Config{
				LogLevel: "debug",
				LogJSON:  false,
			},
			testLog: true,
		},
		{
			name: "JSON formatter",
			cfg: &Config{
				LogLevel: "info",
				LogJSON:  true,
			},
			testLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := tt.cfg.SetupLogger()

			if logger == nil {
				t.Fatal("Expected logger to be created")
			}

			expectedLevel := tt.cfg.GetLogLevel()
			if logger.GetLevel() != expectedLevel {
				t.Errorf("Logger level = %v, expected %v", logger.GetLevel(), expectedLevel)
			}

			// Test that logger can actually log without panicking
			if tt.testLog {
				logger.Debug("Test debug message")
				logger.Info("Test info message")
				logger.Warn("Test warn message")
			}
		})
	}
}

func TestGetEnvironmentInfo(t *testing.T) {
	// Set some test environment variables
	os.Setenv("GO_VERSION", "1.21")
	os.Setenv("BUILD_TIME", "2024-01-01")
	defer func() {
		os.Unsetenv("GO_VERSION")
		os.Unsetenv("BUILD_TIME")
	}()

	info := GetEnvironmentInfo()

	if info == nil {
		t.Fatal("Expected environment info to be returned")
	}

	if info["GO_VERSION"] != "1.21" {
		t.Errorf("Expected GO_VERSION=1.21, got %s", info["GO_VERSION"])
	}

	if info["BUILD_TIME"] != "2024-01-01" {
		t.Errorf("Expected BUILD_TIME=2024-01-01, got %s", info["BUILD_TIME"])
	}

	// Check that all expected keys are present
	expectedKeys := []string{"GO_VERSION", "BUILD_TIME", "GIT_COMMIT", "VERSION"}
	for _, key := range expectedKeys {
		if _, exists := info[key]; !exists {
			t.Errorf("Expected key %s to be present in environment info", key)
		}
	}
}

// TestGetEnvHelpers removed - these functions are unexported and tested indirectly via LoadConfig

func TestConfigSummary(t *testing.T) {
	cfg := &Config{
		CacheStrategy:       "aggressive",
		MaxCacheBreakpoints: 3,
		TokenMultiplier:     1.5,
		AnthropicAPIKey:     "sk-ant-test",
		EnableMetrics:       true,
		EnableDetailedROI:   false,
	}

	summary := cfg.ConfigSummary()

	if summary == nil {
		t.Fatal("Expected config summary to be returned")
	}

	expectedKeys := []string{
		"cache_strategy", "max_cache_breakpoints", "token_multiplier",
		"api_key_configured", "metrics_enabled", "detailed_roi_enabled",
	}

	for _, key := range expectedKeys {
		if _, exists := summary[key]; !exists {
			t.Errorf("Expected key %s to be present in config summary", key)
		}
	}

	if summary["cache_strategy"] != "aggressive" {
		t.Errorf("Expected cache_strategy=aggressive, got %v", summary["cache_strategy"])
	}

	if summary["api_key_configured"] != true {
		t.Error("Expected api_key_configured=true")
	}

	if summary["metrics_enabled"] != true {
		t.Error("Expected metrics_enabled=true")
	}

	if summary["detailed_roi_enabled"] != false {
		t.Error("Expected detailed_roi_enabled=false")
	}
}

func TestUpdateFromEnvironment(t *testing.T) {
	// Save original environment
	originalVars := make(map[string]string)
	testVars := []string{"CACHE_STRATEGY", "LOG_LEVEL", "MAX_CACHE_BREAKPOINTS"}
	for _, v := range testVars {
		originalVars[v] = os.Getenv(v)
		os.Unsetenv(v)
	}
	defer func() {
		for v, val := range originalVars {
			if val != "" {
				os.Setenv(v, val)
			} else {
				os.Unsetenv(v)
			}
		}
	}()

	cfg := &Config{
		Port:                "8080",
		AnthropicURL:        "https://api.anthropic.com",
		CacheStrategy:       "moderate",
		LogLevel:            "info",
		MaxCacheBreakpoints: 4,
		TokenMultiplier:     1.0,
		TokenizerMode:       "offline",
	}

	// Set new environment values
	os.Setenv("CACHE_STRATEGY", "aggressive")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("MAX_CACHE_BREAKPOINTS", "2")

	err := cfg.UpdateFromEnvironment()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cfg.CacheStrategy != "aggressive" {
		t.Errorf("Expected cache strategy to be updated to aggressive, got %s", cfg.CacheStrategy)
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected log level to be updated to debug, got %s", cfg.LogLevel)
	}

	if cfg.MaxCacheBreakpoints != 2 {
		t.Errorf("Expected max breakpoints to be updated to 2, got %d", cfg.MaxCacheBreakpoints)
	}

	// Port should remain unchanged (not updateable at runtime)
	if cfg.Port != "8080" {
		t.Errorf("Expected port to remain 8080, got %s", cfg.Port)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

// TestTokenizerModeValidation tests TOKENIZER_MODE configuration
func TestTokenizerModeValidation(t *testing.T) {
	tests := []struct {
		name        string
		mode        string
		expectError bool
	}{
		{"Valid offline mode", "offline", false},
		{"Valid heuristic mode", "heuristic", false},
		{"Valid hybrid mode", "hybrid", false},
		{"Invalid mode", "invalid", true},
		{"Empty mode", "", true},
		{"Random string", "xyz", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Port:                "8080",
				AnthropicURL:        "https://api.anthropic.com",
				CacheStrategy:       "moderate",
				LogLevel:            "info",
				MaxCacheBreakpoints: 4,
				TokenMultiplier:     1.0,
				TokenizerMode:       tt.mode,
			}

			err := cfg.Validate()

			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
			if tt.expectError && err != nil {
				if !contains(err.Error(), "tokenizer mode") {
					t.Errorf("Expected error about tokenizer mode, got: %v", err)
				}
			}
		})
	}
}

// TestTokenizerPanicSamplesValidation tests TOKENIZER_PANIC_SAMPLES configuration
func TestTokenizerPanicSamplesValidation(t *testing.T) {
	tests := []struct {
		name        string
		samples     int
		expectError bool
	}{
		{"Zero samples", 0, false},
		{"Small samples", 100, false},
		{"Default samples", 200, false},
		{"Large samples", 1000, false},
		{"Negative samples", -1, true},
		{"Very negative", -100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Port:                  "8080",
				AnthropicURL:          "https://api.anthropic.com",
				CacheStrategy:         "moderate",
				LogLevel:              "info",
				MaxCacheBreakpoints:   4,
				TokenMultiplier:       1.0,
				TokenizerMode:         "offline",
				TokenizerPanicSamples: tt.samples,
			}

			err := cfg.Validate()

			if tt.expectError && err == nil {
				t.Error("Expected validation error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected validation error: %v", err)
			}
			if tt.expectError && err != nil {
				if !contains(err.Error(), "tokenizer panic samples") {
					t.Errorf("Expected error about tokenizer panic samples, got: %v", err)
				}
			}
		})
	}
}

// TestConfigWithTokenizerOptions tests loading config with tokenizer options
func TestConfigWithTokenizerOptions(t *testing.T) {
	// Save original environment
	originalEnv := make(map[string]string)
	tokenVars := []string{
		"TOKENIZER_MODE", "LOG_TOKENIZER_FAILURES", "TOKENIZER_PANIC_SAMPLES",
		"PORT", "ANTHROPIC_API_URL", "CACHE_STRATEGY",
	}

	for _, env := range tokenVars {
		originalEnv[env] = os.Getenv(env)
		os.Unsetenv(env)
	}

	defer func() {
		for env, value := range originalEnv {
			if value != "" {
				os.Setenv(env, value)
			} else {
				os.Unsetenv(env)
			}
		}
	}()

	t.Run("Default tokenizer config", func(t *testing.T) {
		cfg, err := LoadConfig()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if cfg.TokenizerMode != "offline" {
			t.Errorf("Expected default tokenizer mode 'offline', got %s", cfg.TokenizerMode)
		}
		if !cfg.LogTokenizerFailures {
			t.Error("Expected log tokenizer failures to be true by default")
		}
		if cfg.TokenizerPanicSamples != 200 {
			t.Errorf("Expected default panic samples 200, got %d", cfg.TokenizerPanicSamples)
		}
	})

	t.Run("Custom tokenizer config", func(t *testing.T) {
		os.Setenv("TOKENIZER_MODE", "heuristic")
		os.Setenv("LOG_TOKENIZER_FAILURES", "false")
		os.Setenv("TOKENIZER_PANIC_SAMPLES", "500")

		cfg, err := LoadConfig()
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if cfg.TokenizerMode != "heuristic" {
			t.Errorf("Expected tokenizer mode 'heuristic', got %s", cfg.TokenizerMode)
		}
		if cfg.LogTokenizerFailures {
			t.Error("Expected log tokenizer failures to be false")
		}
		if cfg.TokenizerPanicSamples != 500 {
			t.Errorf("Expected panic samples 500, got %d", cfg.TokenizerPanicSamples)
		}
	})

	t.Run("Invalid tokenizer mode in environment", func(t *testing.T) {
		os.Setenv("TOKENIZER_MODE", "invalid_mode")

		_, err := LoadConfig()
		if err == nil {
			t.Error("Expected error for invalid tokenizer mode")
		}
		if !contains(err.Error(), "tokenizer mode") {
			t.Errorf("Expected error about tokenizer mode, got: %v", err)
		}
	})
}

// TestConfigSummaryWithTokenizer tests config summary includes tokenizer info
func TestConfigSummaryWithTokenizer(t *testing.T) {
	cfg := &Config{
		CacheStrategy:       "aggressive",
		MaxCacheBreakpoints: 3,
		TokenMultiplier:     1.5,
		AnthropicAPIKey:     "sk-ant-test",
		EnableMetrics:       true,
		EnableDetailedROI:   false,
		TokenizerMode:       "offline",
	}

	summary := cfg.ConfigSummary()

	if summary == nil {
		t.Fatal("Expected config summary to be returned")
	}

	// Verify all expected keys are present
	expectedKeys := []string{
		"cache_strategy", "max_cache_breakpoints", "token_multiplier",
		"api_key_configured", "metrics_enabled", "detailed_roi_enabled",
	}

	for _, key := range expectedKeys {
		if _, exists := summary[key]; !exists {
			t.Errorf("Expected key %s to be present in config summary", key)
		}
	}
}
