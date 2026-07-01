package main

import "fmt"

// IntelligenceTier controls the sophistication level of task execution.
// Tier 0 is the default deterministic mode; higher tiers activate progressively
// more powerful (and riskier) strategies.
type IntelligenceTier int

const (
	Tier0Deterministic IntelligenceTier = iota // default: deterministic micro-edit
	Tier1MultiFile                             // coordinated multi-file edit
	Tier2Architectural                         // architectural planning mode
	Tier3Speculative                           // experimental speculative mode
)

// activeTier holds the current execution tier. It is reset to Tier0 after
// each task completes (de-escalation). The orchestrator is single-threaded
// in task execution, so this package-level variable is safe.
var activeTier = Tier0Deterministic

// patchCapMultiplier returns the patch-line multiplier for the active tier.
// Tier 0 = 1.0× (unchanged), escalating up to 2.0× for Tier 2.
func patchCapMultiplier() float64 {
	switch activeTier {
	case Tier1MultiFile:
		return 1.4
	case Tier2Architectural:
		return 2.0
	default:
		return 1.0
	}
}

// fileCapBonus returns additional files allowed beyond maxFilesTouched.
func fileCapBonus() int {
	if activeTier >= Tier1MultiFile {
		return 2
	}
	return 0
}

// maybeEscalateTier checks escalation triggers and promotes activeTier if warranted.
// Guardrails cap escalation at Tier2 unless --speculative was explicitly passed.
func maybeEscalateTier(task *Task, stats *executionStats) {
	next := activeTier
	switch {
	case task.RetryCount >= 3:
		next = activeTier + 1
	case stats.stability.oscillationCount >= 3:
		next = activeTier + 1
	}
	cap := Tier2Architectural
	if speculativeMode {
		cap = Tier3Speculative
	}
	if next > cap {
		next = cap
	}
	if next != activeTier {
		logInfo("tier_escalated", task.ID, fmt.Sprintf(
			"tier %d → %d (retries=%d oscillations=%d)",
			activeTier, next, task.RetryCount, stats.stability.oscillationCount,
		))
		activeTier = next
	}
}

// deescalateTier resets activeTier to Tier0 after a task completes or is blocked.
// This implements the automatic de-escalation requirement.
func deescalateTier(taskID string) {
	if activeTier != Tier0Deterministic {
		logInfo("tier_deescalated", taskID, fmt.Sprintf("tier %d → 0", activeTier))
		activeTier = Tier0Deterministic
	}
}
