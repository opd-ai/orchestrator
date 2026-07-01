package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opd-ai/orchestrator/memory"
)

func TestAllowedPatchLinesUsesDefaultWithoutMetrics(t *testing.T) {
	maxPatchLines = 50
	selfEvolve = false

	task := &Task{Description: "Update planner", RetryCount: 0}
	if got := allowedPatchLines(task); got != 50 {
		t.Fatalf("allowedPatchLines() = %d, want 50", got)
	}
}

func TestAdaptivePatchBaseClampsFromHistoricalAverage(t *testing.T) {
	maxPatchLines = 50
	selfEvolve = false
	withAdaptiveMetrics(t, memory.AdaptiveMetrics{
		AvgSuccessPatchSize: 20,
		TotalRuns:           2,
	})

	task := &Task{Description: "Update planner", RetryCount: 0}
	if got := allowedPatchLines(task); got != 25 {
		t.Fatalf("allowedPatchLines() = %d, want 25", got)
	}
}

func TestAdaptivePatchBaseClampsUpperBound(t *testing.T) {
	maxPatchLines = 50
	selfEvolve = false
	withAdaptiveMetrics(t, memory.AdaptiveMetrics{
		AvgSuccessPatchSize: 200,
		TotalRuns:           2,
	})

	task := &Task{Description: "Update planner", RetryCount: 0}
	if got := allowedPatchLines(task); got != 80 {
		t.Fatalf("allowedPatchLines() = %d, want 80", got)
	}
}

func TestAllowedPatchLinesSelfEvolveRespectsMinimum(t *testing.T) {
	maxPatchLines = 50
	selfEvolve = true
	withAdaptiveMetrics(t, memory.AdaptiveMetrics{
		AvgSuccessPatchSize: 20,
		TotalRuns:           2,
	})

	task := &Task{Description: "Improve orchestrator planner", RetryCount: 0}
	if got := allowedPatchLines(task); got != 150 {
		t.Fatalf("allowedPatchLines() = %d, want 150", got)
	}
}

func withAdaptiveMetrics(t *testing.T, metrics memory.AdaptiveMetrics) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", tmpDir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("failed to restore cwd: %v", err)
		}
	})

	dataPath := filepath.Join(tmpDir, memory.MetricsFile)
	if err := memory.SaveMetrics(metrics); err != nil {
		t.Fatalf("SaveMetrics(%q) error = %v", dataPath, err)
	}
}
