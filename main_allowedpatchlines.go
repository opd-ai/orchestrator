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

	// Elevated mode for orchestrator self-modification
	if selfEvolve && strings.Contains(strings.ToLower(task.Description), "orchestrator") && base < 150 {
		base = 150
	}

	// Apply tier guardrail: scale base by active intelligence tier multiplier.
	base = lineLimit(int(float64(base) * patchCapMultiplier()))

	// Merged tasks earn proportionally larger patch budgets (capped at 2×).
	if task.MergedCount > 1 {
		factor := min(task.MergedCount, 2)
		base = lineLimit(base * factor)
	}

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
