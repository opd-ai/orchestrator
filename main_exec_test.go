package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordRetryConvergenceTracksRepeatedFailures(t *testing.T) {
	stats := executionStats{}
	stats.recordRetryConvergence("R1", 1, "undefined symbol", "undefined symbol")
	if stats.convergenceSamples != 0 || stats.convergenceAlerts != 0 {
		t.Fatalf("unexpected counts before threshold: %#v", stats)
	}

	stats.recordRetryConvergence("R1", 2, "undefined symbol", "undefined symbol")
	if stats.convergenceSamples != 1 || stats.convergenceAlerts != 1 {
		t.Fatalf("expected one alert, got samples=%d alerts=%d", stats.convergenceSamples, stats.convergenceAlerts)
	}

	stats.recordRetryConvergence("R1", 3, "undefined symbol", "type mismatch")
	if stats.convergenceSamples != 2 || stats.convergenceAlerts != 1 {
		t.Fatalf("expected convergence sample without alert, got samples=%d alerts=%d", stats.convergenceSamples, stats.convergenceAlerts)
	}
}

func TestRevertBuildFailurePatchesRevertsFixesAndOriginal(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	path := filepath.Join(tmpDir, "sample.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	originalDiff := strings.Join([]string{
		"diff --git a/sample.txt b/sample.txt",
		"--- a/sample.txt",
		"+++ b/sample.txt",
		"@@ -1 +1 @@",
		"-a",
		"+b",
	}, "\n") + "\n"
	fixDiff := strings.Join([]string{
		"diff --git a/sample.txt b/sample.txt",
		"--- a/sample.txt",
		"+++ b/sample.txt",
		"@@ -1 +1 @@",
		"-b",
		"+c",
	}, "\n") + "\n"

	if err := applyPatch(originalDiff); err != nil {
		t.Fatalf("apply original diff: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after original patch: %v", err)
	}
	if got := string(data); got != "b\n" {
		t.Fatalf("expected original patch to update contents to b, got %q", got)
	}

	if err := applyPatch(fixDiff); err != nil {
		t.Fatalf("apply fix diff: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after fix patch: %v", err)
	}
	if got := string(data); got != "c\n" {
		t.Fatalf("expected fix patch to update contents to c, got %q", got)
	}

	if err := revertBuildFailurePatches(originalDiff, []string{fixDiff}); err != nil {
		t.Fatalf("revert build failure patches: %v", err)
	}

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got := string(data); got != "a\n" {
		t.Fatalf("expected file restored to original contents, got %q", got)
	}
}
