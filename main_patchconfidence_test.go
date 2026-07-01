package main

import (
	"strings"
	"testing"
)

func TestEvaluatePatchConfidencePenalizesDeletionAndChurn(t *testing.T) {
	compact := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")
	highChurn := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,4 +1 @@",
		"-a", "-b", "-c", "-d",
		"+z",
		"diff --git a/main_exec.go b/main_exec.go",
		"--- a/main_exec.go",
		"+++ b/main_exec.go",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")

	lowRisk := evaluatePatchConfidence(compact)
	risky := evaluatePatchConfidence(highChurn)
	if risky.score >= lowRisk.score {
		t.Fatalf("expected risky patch score < compact score, got %.2f >= %.2f", risky.score, lowRisk.score)
	}
}

func TestPatchConfidenceMessageIncludesSignals(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")

	msg := evaluatePatchConfidence(diff).message()
	for _, want := range []string{"score=", "entropy=", "deletion_ratio=", "structural_churn="} {
		if !strings.Contains(msg, want) {
			t.Fatalf("expected %q in %q", want, msg)
		}
	}
}
