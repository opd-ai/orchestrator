package main

import (
	"math"
	"strings"

	"github.com/opd-ai/orchestrator/memory"
)

const (
	absoluteMinPatchLines = 10
	absoluteMaxPatchLines = 360
)

func lineLimit(in int) int {
	if in < absoluteMinPatchLines {
		return absoluteMinPatchLines
	}
	if in > absoluteMaxPatchLines {
		return absoluteMaxPatchLines
	}
	return in
}

func adaptivePatchBase(defaultBase int) int {
	m, err := memory.LoadMetricsFromBranch()
	if err != nil || m.TotalRuns == 0 || m.AvgSuccessPatchSize <= 0 {
		return lineLimit(defaultBase)
	}

	minSafe := max(absoluteMinPatchLines, int(math.Round(float64(defaultBase)*0.5)))
	maxSafe := lineLimit(int(math.Round(float64(defaultBase) * 1.6)))
	derived := lineLimit(int(math.Round(m.AvgSuccessPatchSize)))

	if derived < minSafe {
		return minSafe
	}
	if derived > maxSafe {
		return maxSafe
	}
	return derived
}

func allowedPatchLines(task *Task) int {
	base := adaptivePatchBase(maxPatchLines)
	if selfEvolve && strings.Contains(strings.ToLower(task.Description), "orchestrator") && base < 150 {
		base = 150
	}
	base = applyScalingFactors(base, task)

	// Adaptive escalation by retry count
	switch task.RetryCount {
	case 0:
		return lineLimit(base)
	case 1:
		return lineLimit(int(float64(base) * 1.4))
	case 2:
		return lineLimit(int(float64(base) * 1.8))
	default:
		return lineLimit(int(float64(base) * 2.5))
	}
}

// applyScalingFactors applies tier, subsystem stability, and merge-count
// multipliers to the base patch budget.
func applyScalingFactors(base int, task *Task) int {
	base = lineLimit(int(float64(base) * patchCapMultiplier()))
	base = lineLimit(int(float64(base) * subsystemBudgetMultiplier(taskSubsystem(task))))
	if task.MergedCount > 1 {
		base = lineLimit(base * min(task.MergedCount, 2))
	}
	return base
}
