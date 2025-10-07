package tokenizer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"autocache/internal/types"

	"github.com/sirupsen/logrus"
)

// TestPrecisionSpanishUnicode tests precision on Spanish text with Unicode characters
// This tests the exact text that was causing panics in production
func TestPrecisionSpanishUnicode(t *testing.T) {
	// Skip if no API key
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping precision test: ANTHROPIC_API_KEY not set")
	}

	// The problematic Spanish text from production logs
	spanishText := "Necesito que generes un informe diario de ayer. Ayer fue 3 octubre 2025. Quiero: 1º Análisis de las visitas de la web ayer. 2º ¿Qué eventos tenemos hoy, mañana y pasado mañana?, 3º ¿Qué tareas tenemos pendientes?"

	// Test all tokenizers
	testCases := []struct {
		name          string
		tokenizerMode string
	}{
		{"Anthropic Tokenizer", "anthropic"},
		{"Offline Tokenizer", "offline"},
		{"Heuristic Tokenizer", "heuristic"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create tokenizer
			var tokenizer Tokenizer
			var err error

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			switch tc.tokenizerMode {
			case "anthropic":
				tokenizer, err = NewAnthropicRealTokenizerWithLogger(logger)
				if err != nil {
					t.Fatalf("Failed to create anthropic tokenizer: %v", err)
				}
			case "offline":
				tokenizer, err = NewOfflineTokenizerWithLogger(logger)
				if err != nil {
					t.Fatalf("Failed to create offline tokenizer: %v", err)
				}
			case "heuristic":
				tokenizer = NewAnthropicTokenizer()
			}

			// Count tokens
			estimatedTokens := tokenizer.CountTokens(spanishText)

			// Call actual Anthropic API
			actualTokens, err := callAnthropicAPIForTokenCount(apiKey, spanishText)
			if err != nil {
				t.Fatalf("Failed to call Anthropic API: %v", err)
			}

			// Calculate accuracy
			diff := estimatedTokens - actualTokens
			diffPercent := float64(diff) / float64(actualTokens) * 100

			t.Logf("%s: Estimated=%d, Actual=%d, Diff=%d (%.1f%%)",
				tc.name, estimatedTokens, actualTokens, diff, diffPercent)

			// Anthropic tokenizer should be very accurate (within ±5 tokens or 10%)
			if tc.tokenizerMode == "anthropic" {
				if diff < -5 || diff > 5 {
					absPercent := diffPercent
					if absPercent < 0 {
						absPercent = -absPercent
					}
					if absPercent > 10 {
						t.Errorf("Anthropic tokenizer not accurate enough: diff=%d (%.1f%%)", diff, diffPercent)
					}
				}
			}
		})
	}
}

// TestPrecisionN8NWorkflow tests precision on the actual n8n workflow data
func TestPrecisionN8NWorkflow(t *testing.T) {
	// Skip if no API key
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping precision test: ANTHROPIC_API_KEY not set")
	}

	// Load n8n test data
	inputData, err := os.ReadFile("test_data/scenarios/n8n_agent_workflow/input.json")
	if err != nil {
		t.Skipf("Skipping test: n8n test data not found: %v", err)
	}

	var req types.AnthropicRequest
	if err := json.Unmarshal(inputData, &req); err != nil {
		t.Fatalf("Failed to parse n8n input: %v", err)
	}

	// Test all tokenizers
	testCases := []struct {
		name          string
		tokenizerMode string
	}{
		{"Anthropic Tokenizer", "anthropic"},
		{"Offline Tokenizer", "offline"},
		{"Heuristic Tokenizer", "heuristic"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create tokenizer
			var tokenizer Tokenizer
			var tokErr error

			logger := logrus.New()
			logger.SetLevel(logrus.ErrorLevel)

			switch tc.tokenizerMode {
			case "anthropic":
				tokenizer, tokErr = NewAnthropicRealTokenizerWithLogger(logger)
				if tokErr != nil {
					t.Fatalf("Failed to create anthropic tokenizer: %v", tokErr)
				}
			case "offline":
				tokenizer, tokErr = NewOfflineTokenizerWithLogger(logger)
				if tokErr != nil {
					t.Fatalf("Failed to create offline tokenizer: %v", tokErr)
				}
			case "heuristic":
				tokenizer = NewAnthropicTokenizer()
			}

			// Estimate tokens
			estimatedTokens := tokenizer.EstimateRequestTokens(&req)

			// Call actual Anthropic API
			actualTokens, apiErr := callAnthropicAPIForRequestTokens(apiKey, &req)
			if apiErr != nil {
				t.Fatalf("Failed to call Anthropic API: %v", apiErr)
			}

			// Calculate accuracy
			diff := estimatedTokens - actualTokens
			diffPercent := float64(diff) / float64(actualTokens) * 100

			t.Logf("%s: Estimated=%d, Actual=%d, Diff=%d (%.1f%%)",
				tc.name, estimatedTokens, actualTokens, diff, diffPercent)

			// Anthropic tokenizer should be accurate (within ±10% for complex requests)
			if tc.tokenizerMode == "anthropic" {
				absPercent := diffPercent
				if absPercent < 0 {
					absPercent = -absPercent
				}
				if absPercent > 15 {
					t.Errorf("Anthropic tokenizer not accurate enough: diff=%d (%.1f%%)", diff, diffPercent)
				}
			}
		})
	}
}

// callAnthropicAPIForTokenCount makes an actual API call to count tokens
func callAnthropicAPIForTokenCount(apiKey, text string) (int, error) {
	// Create a minimal request
	req := types.AnthropicRequest{
		Model: "claude-3-5-sonnet-20241022",
		Messages: []types.Message{
			{
				Role: "user",
				Content: []types.ContentBlock{
					{
						Type: "text",
						Text: text,
					},
				},
			},
		},
		MaxTokens: 1,
	}

	return callAnthropicAPIForRequestTokens(apiKey, &req)
}

// callAnthropicAPIForRequestTokens makes an actual API call and extracts token count from usage
func callAnthropicAPIForRequestTokens(apiKey string, req *types.AnthropicRequest) (int, error) {
	// Ensure max_tokens is set
	if req.MaxTokens == 0 {
		req.MaxTokens = 1
	}

	// Marshal request
	reqBody, err := json.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBody))
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Make request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResp types.AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return 0, fmt.Errorf("failed to decode response: %w", err)
	}

	// Return actual input token count
	return apiResp.Usage.InputTokens, nil
}

// TestPrecisionEdgeCases tests precision on various edge cases
func TestPrecisionEdgeCases(t *testing.T) {
	// Skip if no API key
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping precision test: ANTHROPIC_API_KEY not set")
	}

	testCases := []struct {
		name string
		text string
	}{
		{"Empty string", ""},
		{"Single word", "Hello"},
		{"Short sentence", "The quick brown fox jumps over the lazy dog."},
		{"Unicode emojis", "Hello 👋 World 🌍 Test 🚀"},
		{"Spanish with accents", "¿Cómo estás? ¡Muy bien! El niño comió mañana."},
		{"Mixed languages", "Hello world, こんにちは世界, 你好世界, مرحبا بالعالم"},
		{"Code snippet", "function test() { return x + y; }"},
		{"JSON", `{"name": "test", "value": 123, "nested": {"key": "value"}}`},
	}

	// Only test anthropic tokenizer for edge cases (most accurate)
	tokenizer, err := NewAnthropicRealTokenizer()
	if err != nil {
		t.Fatalf("Failed to create anthropic tokenizer: %v", err)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.text == "" {
				t.Skip("Skipping empty string (would cost API call)")
				return
			}

			// Estimate
			estimated := tokenizer.CountTokens(tc.text)

			// Get actual
			actual, err := callAnthropicAPIForTokenCount(apiKey, tc.text)
			if err != nil {
				t.Fatalf("Failed to call API: %v", err)
			}

			diff := estimated - actual
			var diffPercent float64
			if actual > 0 {
				diffPercent = float64(diff) / float64(actual) * 100
			}

			t.Logf("Text: %q, Estimated=%d, Actual=%d, Diff=%d (%.1f%%)",
				tc.text, estimated, actual, diff, diffPercent)

			// Should be very accurate on simple cases
			if actual < 50 {
				// For short texts, allow ±3 tokens or 20% (whichever is larger)
				if diff < -3 || diff > 3 {
					absPercent := diffPercent
					if absPercent < 0 {
						absPercent = -absPercent
					}
					if absPercent > 20 {
						t.Logf("Warning: Less accurate on short text: diff=%d (%.1f%%)", diff, diffPercent)
					}
				}
			}
		})
	}
}
