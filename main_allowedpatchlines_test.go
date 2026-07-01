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

func TestApplyScalingFactorsTierMultiplier(t *testing.T) {
	t.Cleanup(func() { activeTier = Tier0Deterministic })

	task := &Task{Description: "Update planner", RetryCount: 0}

	activeTier = Tier0Deterministic
	if got := applyScalingFactors(100, task); got != 100 {
		t.Fatalf("Tier0: applyScalingFactors(100) = %d, want 100", got)
	}

	activeTier = Tier1MultiFile
	// 100 * 1.4 = 140
	if got := applyScalingFactors(100, task); got != 140 {
		t.Fatalf("Tier1: applyScalingFactors(100) = %d, want 140", got)
	}

	activeTier = Tier2Architectural
	// 100 * 2.0 = 200
	if got := applyScalingFactors(100, task); got != 200 {
		t.Fatalf("Tier2: applyScalingFactors(100) = %d, want 200", got)
	}
}

func TestApplyScalingFactorsSubsystemMultiplier(t *testing.T) {
	activeTier = Tier0Deterministic
	task := &Task{Description: "Update planner", Files: []string{"audit/main.go"}, RetryCount: 0}

	subsystem := taskSubsystem(task)

	// Register an unstable subsystem: 0.70×
	subsystemRegistry[subsystem] = &subsystemMetrics{successes: 1, failures: 4, patchCount: 5}
	// 100 * 0.70 = 70
	if got := applyScalingFactors(100, task); got != 70 {
		t.Fatalf("unstable: applyScalingFactors(100) = %d, want 70", got)
	}

	// Register a stable subsystem: 1.20×
	subsystemRegistry[subsystem] = &subsystemMetrics{successes: 6, failures: 0, patchCount: 6}
	// 100 * 1.20 = 120
	if got := applyScalingFactors(100, task); got != 120 {
		t.Fatalf("stable: applyScalingFactors(100) = %d, want 120", got)
	}

	// Clean up
	delete(subsystemRegistry, subsystem)
}

func TestApplyScalingFactorsMergedTaskMultiplier(t *testing.T) {
	activeTier = Tier0Deterministic
	// Use a subsystem with no registry entry to keep multiplier at 1.0.
	task := &Task{Description: "Update planner", Files: []string{"unregistered/x.go"}, RetryCount: 0}

	task.MergedCount = 2
	// 100 * min(2,2) = 200
	if got := applyScalingFactors(100, task); got != 200 {
		t.Fatalf("merged 2: applyScalingFactors(100) = %d, want 200", got)
	}

	task.MergedCount = 5
	// capped at min(5,2) = 2 → 200
	if got := applyScalingFactors(100, task); got != 200 {
		t.Fatalf("merged 5 (capped): applyScalingFactors(100) = %d, want 200", got)
	}

	task.MergedCount = 1
	// No multiplier applied for MergedCount <= 1
	if got := applyScalingFactors(100, task); got != 100 {
		t.Fatalf("merged 1 (no multiplier): applyScalingFactors(100) = %d, want 100", got)
	}
}
