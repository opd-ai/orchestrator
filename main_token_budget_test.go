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

func TestEnforceTokenBudgetPreservesNewlines(t *testing.T) {
	prefix := "EXECUTION_BLOCK\nMODE: EXECUTE\nTASK_ID: T1\n"
	var b strings.Builder
	b.WriteString(prefix)
	for i := 0; i < maxPromptTokens+25; i++ {
		b.WriteString("token ")
	}

	got := enforceTokenBudget(b.String())
	if !strings.Contains(got, "\nMODE: EXECUTE\n") {
		t.Fatalf("expected newlines to be preserved, got %q", got)
	}
}
