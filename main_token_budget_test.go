package main

import (
	"strings"
	"testing"
)

func TestEnforceTokenBudgetTruncates(t *testing.T) {
	var b strings.Builder
	for i := 0; i < maxPromptChars+25; i++ {
		b.WriteRune('a')
	}

	got := enforceTokenBudget(b.String())
	if n := len([]rune(got)); n != maxPromptChars {
		t.Fatalf("expected %d chars, got %d", maxPromptChars, n)
	}
}

func TestEnforceTokenBudgetPreservesNewlines(t *testing.T) {
	prefix := "EXECUTION_BLOCK\nMODE: EXECUTE\nTASK_ID: T1\n"
	var b strings.Builder
	b.WriteString(prefix)
	for i := 0; i < maxPromptChars+25; i++ {
		b.WriteRune('b')
	}

	got := enforceTokenBudget(b.String())
	if !strings.Contains(got, "\nMODE: EXECUTE\n") {
		t.Fatalf("expected newlines to be preserved, got %q", got)
	}
}
