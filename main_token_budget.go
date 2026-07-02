package main

const (
	maxPromptTokens = 1500
	maxPromptChars  = maxPromptTokens * 4
)

// enforceTokenBudget approximates model token limits using a character cap.
// For Go-heavy prompts, ~4 characters per token is a conservative heuristic,
// so 1500 tokens maps to ~6000 characters. This can still drift from true BPE
// counts; if stricter limits are required, use a model-specific tokenizer.
func enforceTokenBudget(prompt string) string {
	runes := []rune(prompt)
	if len(runes) <= maxPromptChars {
		return prompt
	}
	return string(runes[:maxPromptChars])
}
