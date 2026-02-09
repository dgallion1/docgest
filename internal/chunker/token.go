package chunker

import "strings"

// EstimateTokens gives a rough token count using the ~4 chars/token heuristic.
// This is intentionally simple — exact tokenization is not required for chunking.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	// Count words as a better proxy than pure character division.
	words := len(strings.Fields(text))
	// Roughly 0.75 tokens per word for English text.
	tokens := int(float64(words) * 1.33)
	if tokens < 1 && len(text) > 0 {
		tokens = 1
	}
	return tokens
}
