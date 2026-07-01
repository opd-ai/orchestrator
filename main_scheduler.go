package main

import (
	"fmt"
	"os"
	"runtime"
)

// ── Reward Scoring Engine ─────────────────────────────────────────────────────

// taskReward captures the outcome quality of a completed task.
type taskReward struct {
	taskID    string
	retries   int
	patchSize int
	score     float64
}

// computeReward returns a [0,1] quality score for a task outcome.
// Higher score = better (fewer retries, smaller patch, no oscillation).
func computeReward(taskID string, retries, patchSize int) taskReward {
	retryPenalty := clamp01(float64(retries) * 0.15)
	sizePenalty := clamp01(float64(patchSize) / float64(maxPatchLines+1))
	score := clamp01(1.0 - retryPenalty - (0.20 * sizePenalty))
	logInfo("task_reward", taskID, fmt.Sprintf(
		"score=%.2f retries=%d patch_size=%d", score, retries, patchSize,
	))
	return taskReward{taskID: taskID, retries: retries, patchSize: patchSize, score: score}
}

// ── Hardware-Aware Scheduler ──────────────────────────────────────────────────

// cpuLoadFactor returns a [0,1] normalised 1-minute CPU load average.
// Falls back to 0.5 when /proc/loadavg is unavailable (non-Linux platforms).
func cpuLoadFactor() float64 {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0.5
	}
	var load1 float64
	fmt.Sscanf(string(data), "%f", &load1)
	return clamp01(load1 / float64(runtime.NumCPU()))
}

// speculativeCandidateCount returns the number of parallel LLM candidates
// to generate, reduced to 1 under high CPU load to protect throughput.
func speculativeCandidateCount() int {
	if cpuLoadFactor() > 0.80 {
		logInfo("hw_scheduler", "", "high CPU load — reducing speculative candidates to 1")
		return 1
	}
	return len(tempStrategy())
}

// ── Long-Run Stability Monitor ────────────────────────────────────────────────

const (
	safeModeTriggerConsecutiveBlocked = 5
	safeModeTriggerOscillations       = 10
)

// stabilityMonitor tracks signs of instability during a long run.
type stabilityMonitor struct {
	consecutiveBlocked int
	oscillationCount   int
	SafeMode           bool
}

// recordBlock increments the consecutive-blocked counter and may activate safe mode.
func (m *stabilityMonitor) recordBlock() {
	m.consecutiveBlocked++
	if !m.SafeMode && m.consecutiveBlocked >= safeModeTriggerConsecutiveBlocked {
		m.SafeMode = true
		logInfo("safe_mode_activated", "", fmt.Sprintf(
			"%d consecutive blocked tasks", m.consecutiveBlocked,
		))
	}
}

// recordSuccess resets the consecutive-blocked counter.
func (m *stabilityMonitor) recordSuccess() {
	m.consecutiveBlocked = 0
}

// recordOscillation notes a retry-convergence event and may activate safe mode.
func (m *stabilityMonitor) recordOscillation() {
	m.oscillationCount++
	if !m.SafeMode && m.oscillationCount >= safeModeTriggerOscillations {
		m.SafeMode = true
		logInfo("safe_mode_activated", "", fmt.Sprintf(
			"%d oscillation events detected", m.oscillationCount,
		))
	}
}
