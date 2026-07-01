package main

import "strings"

const maxPromptTokens = 1500

func enforceTokenBudget(prompt string) string {
	tokens := strings.Fields(prompt)
	if len(tokens) <= maxPromptTokens {
		return prompt
	}
	return strings.Join(tokens[:maxPromptTokens], " ")
}
