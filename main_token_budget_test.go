package main

import (
	"strings"
	"testing"
)

func TestEnforceTokenBudgetTruncates(t *testing.T) {
	var b strings.Builder
	for i := 0; i < maxPromptTokens+25; i++ {
		b.WriteString("token ")
	}

	got := enforceTokenBudget(b.String())
	if n := len(strings.Fields(got)); n != maxPromptTokens {
		t.Fatalf("expected %d tokens, got %d", maxPromptTokens, n)
	}
}
