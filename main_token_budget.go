package main

import "strings"

const maxPromptTokens = 1500

func enforceTokenBudget(prompt string) string {
	tokenCount := 0
	inToken := false
	cut := len(prompt)

	for i, r := range prompt {
		if strings.ContainsRune(" \t\r\n\f\v", r) {
			inToken = false
			continue
		}
		if inToken {
			continue
		}

		tokenCount++
		if tokenCount > maxPromptTokens {
			cut = i
			break
		}
		inToken = true
	}

	if tokenCount <= maxPromptTokens {
		return prompt
	}
	return prompt[:cut]
}
