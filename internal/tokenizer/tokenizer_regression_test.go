package tokenizer

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"

	"autocache/internal/types"

	"github.com/sirupsen/logrus"
)

// TestUnicodeOrdinalIndicators tests Spanish ordinal indicators that caused the original panic
func TestUnicodeOrdinalIndicators(t *testing.T) {
	testCases := []struct {
		name string
		text string
	}{
		{"Masculine ordinal", "1º paso"},
		{"Feminine ordinal", "1ª vez"},
		{"Multiple ordinals", "1º paso, 2º intento, 3º lugar"},
		{"Production text", "Necesito que generes un informe diario de ayer. Ayer fue 3 octubre 2025. Quiero: 1º Análisis de las visitas de la web ayer. 2º ¿Qué eventos tenemos hoy, mañana y pasado mañana?, 3º ¿Qué tareas tenemos pendientes?"},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	// Test with all tokenizers
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

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for name, tokenizer := range tokenizers {
				// Should not panic
				count := tokenizer.CountTokens(tc.text)

				if count <= 0 {
					t.Errorf("%s: got non-positive count %d for %q", name, count, tc.text)
				}

				t.Logf("%s: %q = %d tokens", name, truncateForLog(tc.text, 50), count)
			}

			// Check offline tokenizer panic stats
			if offlineTokenizer, ok := tokenizers["offline"].(*OfflineTokenizer); ok {
				stats := offlineTokenizer.GetPanicStats()
				if stats["panic_count"] > 0 {
					t.Logf("Note: Offline tokenizer had %d panics (recovered with fallback)", stats["panic_count"])
				}
			}
		})
	}
}

// TestUnicodeAccentedCharacters tests Spanish accented characters
func TestUnicodeAccentedCharacters(t *testing.T) {
	testTexts := []string{
		"á é í ó ú",
		"Á É Í Ó Ú",
		"ñ Ñ",
		"¿Cómo estás?",
		"¡Muy bien!",
		"El niño comió mañana en el jardín.",
		"José María González Pérez",
		"Información técnica y análisis crítico",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	// Test anthropic tokenizer (should never panic)
	anthropic, err := NewAnthropicRealTokenizerWithLogger(logger)
	if err != nil {
		t.Skipf("Skipping test: failed to create tokenizer: %v", err)
	}

	for _, text := range testTexts {
		t.Run(text, func(t *testing.T) {
			// Should not panic
			count := anthropic.CountTokens(text)

			if count <= 0 {
				t.Errorf("Got non-positive count %d for %q", count, text)
			}

			t.Logf("%q = %d tokens", text, count)
		})
	}
}

// TestUnicodeStressTest tests a wide variety of Unicode characters
func TestUnicodeStressTest(t *testing.T) {
	testCases := []struct {
		name string
		text string
	}{
		{"Superscripts", "x¹ x² x³ xⁿ"},
		{"Subscripts", "H₂O CO₂ CH₄"},
		{"Math symbols", "∑ ∫ ∂ ∇ ∞ ≈ ≠ ≤ ≥"},
		{"Greek letters", "α β γ δ ε ζ η θ"},
		{"Currency", "$ € £ ¥ ₹ ₽ ¢"},
		{"Arrows", "← → ↑ ↓ ↔ ⇒ ⇐"},
		{"Emoji simple", "😀 😃 😄 😁 😆"},
		{"Emoji complex", "👨‍💻 👩‍🔬 👨‍👩‍👧‍👦"},
		{"Chinese", "你好世界"},
		{"Japanese", "こんにちは世界"},
		{"Arabic", "مرحبا بالعالم"},
		{"Russian", "Привет мир"},
		{"Mixed", "Hello 世界 🌍 مرحبا Привет"},
	}

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	anthropic, err := NewAnthropicRealTokenizerWithLogger(logger)
	if err != nil {
		t.Skipf("Skipping test: failed to create tokenizer: %v", err)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Should not panic
			count := anthropic.CountTokens(tc.text)

			if count < 0 {
				t.Errorf("Got negative count %d for %q", count, tc.text)
			}

			t.Logf("%s: %q = %d tokens", tc.name, tc.text, count)
		})
	}
}

// TestProductionPanicScenarios tests actual scenarios that caused panics in production
func TestProductionPanicScenarios(t *testing.T) {
	// Real production texts from logs
	productionTexts := []string{
		"Necesito que generes un informe diario de ayer. Ayer fue 3 octubre 2025. Quiero: 1º Análisis de las visitas de la web ayer. 2º ¿Qué eventos tenemos hoy, mañana y pasado mañana?, 3º ¿Qué tareas tenemos pendientes?",
		"Por favor, realiza: 1º Revisión del código, 2º Pruebas unitarias, 3º Documentación",
		"Objetivos: 1º Aumentar ventas, 2º Mejorar satisfacción, 3º Reducir costos",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)

	// Test all tokenizers
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

	for i, text := range productionTexts {
		t.Run(fmt.Sprintf("Production scenario %d", i+1), func(t *testing.T) {
			for name, tokenizer := range tokenizers {
				// Process text (should not panic)
				count := tokenizer.CountTokens(text)

				if count <= 0 {
					t.Errorf("%s: got non-positive count %d", name, count)
				}

				t.Logf("%s: %d tokens", name, count)
			}

			// Verify anthropic tokenizer had 0 panics
			if anthropicTokenizer, ok := tokenizers["anthropic"].(*AnthropicRealTokenizer); ok {
				// Anthropic tokenizer doesn't have panic recovery, so getting here means no panic
				_ = anthropicTokenizer
				t.Logf("Anthropic tokenizer: ✓ No panics")
			}

			// Check offline tokenizer stats
			if offlineTokenizer, ok := tokenizers["offline"].(*OfflineTokenizer); ok {
				stats := offlineTokenizer.GetPanicStats()
				if stats["panic_count"] > 0 {
					t.Logf("Offline tokenizer: %d panics (recovered successfully)", stats["panic_count"])
				} else {
					t.Logf("Offline tokenizer: ✓ No panics")
				}
			}
		})
	}
}

// TestConcurrentUnicodeStress tests concurrent processing of Unicode text
func TestConcurrentUnicodeStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	problematicTexts := []string{
		"1º 2º 3º 4º 5º",
		"á é í ó ú ñ",
		"¿Cómo estás? ¡Muy bien!",
		"你好世界 こんにちは",
		"😀 😃 😄 👨‍💻 👩‍🔬",
	}

	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)

	anthropic, err := NewAnthropicRealTokenizerWithLogger(logger)
	if err != nil {
		t.Skipf("Skipping test: failed to create tokenizer: %v", err)
	}

	const numGoroutines = 100
	const iterationsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	errors := make(chan error, numGoroutines*iterationsPerGoroutine)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterationsPerGoroutine; j++ {
				text := problematicTexts[j%len(problematicTexts)]

				// This should not panic
				count := anthropic.CountTokens(text)

				if count < 0 {
					errors <- fmt.Errorf("goroutine %d iteration %d: got negative count %d for %q",
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
		t.Logf("✓ Successfully processed %d concurrent Unicode operations with 0 panics",
			numGoroutines*iterationsPerGoroutine)
	}
}

// truncateForLog truncates text for logging
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TestRegressionN8NWorkflow verifies the n8n workflow doesn't cause panics with new tokenizer
func TestRegressionN8NWorkflow(t *testing.T) {
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
	logger.SetLevel(logrus.ErrorLevel)

	// Test with anthropic tokenizer (should never panic)
	anthropic, err := NewAnthropicRealTokenizerWithLogger(logger)
	if err != nil {
		t.Skipf("Skipping test: failed to create tokenizer: %v", err)
	}

	// Process the request (should not panic)
	totalTokens := anthropic.EstimateRequestTokens(&req)

	if totalTokens <= 0 {
		t.Errorf("Got non-positive token count: %d", totalTokens)
	}

	t.Logf("✓ N8N workflow processed successfully: %d tokens, 0 panics", totalTokens)

	// Test each tool individually
	t.Logf("Processing %d tools...", len(req.Tools))
	for i, tool := range req.Tools {
		tokens := anthropic.CountToolTokens(tool)
		if tokens <= 0 {
			t.Errorf("Tool %d (%s) has non-positive token count: %d", i, tool.Name, tokens)
		}
	}

	t.Logf("✓ All %d tools processed without panics", len(req.Tools))
}
