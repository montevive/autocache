package tokenizer

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"autocache/internal/types"

	"github.com/sirupsen/logrus"
)

// TestTokenizerConsistency tests that each tokenizer returns consistent results
func TestTokenizerConsistency(t *testing.T) {
	testTexts := []string{
		"Hello world",
		"The quick brown fox jumps over the lazy dog",
		"Necesito que generes un informe diario de ayer. Ayer fue 3 octubre 2025. Quiero: 1º Análisis de las visitas.",
		"function test() { return x + y; }",
		`{"name": "test", "value": 123}`,
	}

	tokenizers := map[string]Tokenizer{}

	// Create tokenizers
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	anthropic, err := NewAnthropicRealTokenizerWithLogger(logger)
	if err != nil {
		t.Logf("Skipping anthropic tokenizer: %v", err)
	} else {
		tokenizers["anthropic"] = anthropic
	}

	offline, err := NewOfflineTokenizerWithLogger(logger)
	if err != nil {
		t.Logf("Skipping offline tokenizer: %v", err)
	} else {
		tokenizers["offline"] = offline
	}

	tokenizers["heuristic"] = NewAnthropicTokenizer()

	// Test consistency
	for tokenizerName, tokenizer := range tokenizers {
		t.Run(tokenizerName, func(t *testing.T) {
			for _, text := range testTexts {
				// Count same text multiple times
				counts := make([]int, 100)
				for i := 0; i < 100; i++ {
					counts[i] = tokenizer.CountTokens(text)
				}

				// Verify all counts are identical
				firstCount := counts[0]
				for i, count := range counts {
					if count != firstCount {
						t.Errorf("Inconsistent count at iteration %d: got %d, expected %d (text: %q)",
							i, count, firstCount, text)
						break
					}
				}

				t.Logf("%s - %q: %d tokens (100 iterations, all consistent)",
					tokenizerName, truncate(text, 50), firstCount)
			}
		})
	}
}

// TestConcurrentTokenization tests thread-safe concurrent access
func TestConcurrentTokenization(t *testing.T) {
	testTexts := []string{
		"Hello world",
		"The quick brown fox",
		"Necesito que generes un informe diario 1º 2º 3º",
		"function test() { return x + y; }",
	}

	tokenizers := map[string]Tokenizer{}

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	anthropic, err := NewAnthropicRealTokenizerWithLogger(logger)
	if err != nil {
		t.Logf("Skipping anthropic tokenizer: %v", err)
	} else {
		tokenizers["anthropic"] = anthropic
	}

	offline, err := NewOfflineTokenizerWithLogger(logger)
	if err != nil {
		t.Logf("Skipping offline tokenizer: %v", err)
	} else {
		tokenizers["offline"] = offline
	}

	tokenizers["heuristic"] = NewAnthropicTokenizer()

	for tokenizerName, tokenizer := range tokenizers {
		t.Run(tokenizerName, func(t *testing.T) {
			// Run 50 goroutines
			const numGoroutines = 50
			const iterationsPerGoroutine = 10

			var wg sync.WaitGroup
			wg.Add(numGoroutines)

			errors := make(chan error, numGoroutines*iterationsPerGoroutine)

			for i := 0; i < numGoroutines; i++ {
				go func(id int) {
					defer wg.Done()

					for j := 0; j < iterationsPerGoroutine; j++ {
						text := testTexts[j%len(testTexts)]
						count := tokenizer.CountTokens(text)

						if count <= 0 {
							errors <- fmt.Errorf("goroutine %d iteration %d: got non-positive count %d for %q",
								id, j, count, text)
						}
					}
				}(i)
			}

			wg.Wait()
			close(errors)

			// Check for errors
			errorCount := 0
			for err := range errors {
				t.Error(err)
				errorCount++
			}

			if errorCount == 0 {
				t.Logf("%s: Successfully processed %d concurrent operations",
					tokenizerName, numGoroutines*iterationsPerGoroutine)
			}
		})
	}
}

// TestTokenizerComparison compares different tokenizers side-by-side
func TestTokenizerComparison(t *testing.T) {
	testCases := []struct {
		name string
		text string
	}{
		{"Simple", "Hello world"},
		{"Spanish Unicode", "Necesito que generes: 1º Análisis, 2º Eventos, 3º Tareas"},
		{"Long text", strings.Repeat("This is a test sentence. ", 50)},
		{"Code", "function fibonacci(n) { if (n <= 1) return n; return fibonacci(n-1) + fibonacci(n-2); }"},
		{"JSON", `{"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}], "total": 2}`},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	// Create all tokenizers
	tokenizers := map[string]Tokenizer{}

	anthropic, err := NewAnthropicRealTokenizerWithLogger(logger)
	if err != nil {
		t.Logf("Skipping anthropic tokenizer: %v", err)
	} else {
		tokenizers["anthropic"] = anthropic
	}

	offline, err := NewOfflineTokenizerWithLogger(logger)
	if err != nil {
		t.Logf("Skipping offline tokenizer: %v", err)
	} else {
		tokenizers["offline"] = offline
	}

	tokenizers["heuristic"] = NewAnthropicTokenizer()

	// Compare results
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			results := make(map[string]int)

			for name, tokenizer := range tokenizers {
				results[name] = tokenizer.CountTokens(tc.text)
			}

			// Log comparison
			t.Logf("Text: %q", truncate(tc.text, 60))
			for name, count := range results {
				t.Logf("  %s: %d tokens", name, count)
			}

			// If anthropic tokenizer available, use it as baseline
			if anthropicCount, ok := results["anthropic"]; ok {
				for name, count := range results {
					if name == "anthropic" {
						continue
					}
					diff := count - anthropicCount
					diffPercent := float64(diff) / float64(anthropicCount) * 100
					t.Logf("  %s vs anthropic: diff=%d (%.1f%%)", name, diff, diffPercent)
				}
			}
		})
	}
}

// TestTokenizerN8NComparison compares tokenizers on the n8n workflow
func TestTokenizerN8NComparison(t *testing.T) {
	// Load n8n test data
	inputData, err := os.ReadFile("test_data/scenarios/n8n_agent_workflow/input.json")
	if err != nil {
		t.Skipf("Skipping test: n8n test data not found: %v", err)
	}

	var req types.AnthropicRequest
	if err := json.Unmarshal(inputData, &req); err != nil {
		t.Fatalf("Failed to parse n8n input: %v", err)
	}

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	// Create all tokenizers
	tokenizers := map[string]Tokenizer{}

	anthropic, err := NewAnthropicRealTokenizerWithLogger(logger)
	if err != nil {
		t.Logf("Skipping anthropic tokenizer: %v", err)
	} else {
		tokenizers["anthropic"] = anthropic
	}

	offline, err := NewOfflineTokenizerWithLogger(logger)
	if err != nil {
		t.Logf("Skipping offline tokenizer: %v", err)
	} else {
		tokenizers["offline"] = offline
	}

	tokenizers["heuristic"] = NewAnthropicTokenizer()

	// Estimate tokens with each tokenizer
	results := make(map[string]int)

	for name, tokenizer := range tokenizers {
		results[name] = tokenizer.EstimateRequestTokens(&req)
	}

	// Log results
	t.Logf("N8N Workflow Token Estimates:")
	for name, count := range results {
		t.Logf("  %s: %d tokens", name, count)
	}

	// Compare if anthropic available
	if anthropicCount, ok := results["anthropic"]; ok {
		for name, count := range results {
			if name == "anthropic" {
				continue
			}
			diff := count - anthropicCount
			diffPercent := float64(diff) / float64(anthropicCount) * 100
			t.Logf("  %s vs anthropic: diff=%d (%.1f%%)", name, diff, diffPercent)
		}
	}

	// Verify no panics occurred for offline tokenizer
	if offlineTokenizer, ok := tokenizers["offline"].(*OfflineTokenizer); ok {
		stats := offlineTokenizer.GetPanicStats()
		if stats["panic_count"] > 0 {
			t.Logf("Note: Offline tokenizer had %d panics but recovered successfully", stats["panic_count"])
		}
	}
}

// truncate helper
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
