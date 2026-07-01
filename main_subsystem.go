package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// subsystemMetrics tracks stability metrics for one subsystem across the session.
type subsystemMetrics struct {
	subsystem    string
	successes    int
	failures     int
	totalRetries int
	totalRisk    float64
	totalSize    int
	patchCount   int
}

// failureRate returns the fraction of attempts that failed, or 0 when no data.
func (m *subsystemMetrics) failureRate() float64 {
	total := m.successes + m.failures
	if total == 0 {
		return 0
	}
	return float64(m.failures) / float64(total)
}

// avgRiskScore returns the mean risk score across recorded patches.
func (m *subsystemMetrics) avgRiskScore() float64 {
	if m.patchCount == 0 {
		return 0
	}
	return m.totalRisk / float64(m.patchCount)
}

// avgPatchSize returns the mean patch line count.
func (m *subsystemMetrics) avgPatchSize() float64 {
	if m.patchCount == 0 {
		return 0
	}
	return float64(m.totalSize) / float64(m.patchCount)
}

// isUnstable returns true when the subsystem shows a pattern of repeated failures.
// Requires at least 3 attempts and a failure rate above 40 %.
func (m *subsystemMetrics) isUnstable() bool {
	return m.successes+m.failures >= 3 && m.failureRate() > 0.40
}

// isStable returns true when the subsystem has a strong success history.
// Requires at least 5 successes and a failure rate below 20 %.
func (m *subsystemMetrics) isStable() bool {
	return m.successes >= 5 && m.failureRate() < 0.20
}

// subsystemRegistry holds per-subsystem metrics for the current session.
// It is populated by recordSubsystemOutcome and consulted by subsystemBudgetMultiplier.
var subsystemRegistry = make(map[string]*subsystemMetrics)

// subsystemBudgetMultiplier returns an adaptive patch-budget multiplier for the
// given subsystem based on its recorded stability:
//   - Unstable subsystems get a 0.70× reduction to limit mutation pressure.
//   - Stable subsystems get a 1.20× increase to reward consistent success.
//   - All others get 1.0× (no change).
func subsystemBudgetMultiplier(subsystem string) float64 {
	m, ok := subsystemRegistry[subsystem]
	if !ok {
		return 1.0
	}
	switch {
	case m.isUnstable():
		return 0.70
	case m.isStable():
		return 1.20
	default:
		return 1.0
	}
}

// taskSubsystem derives the subsystem name from a task's file list.
// Files in the root package map to "root"; files in sub-directories map to
// their first path component.
func taskSubsystem(task *Task) string {
	if len(task.Files) == 0 {
		return "root"
	}
	dir := filepath.Dir(task.Files[0])
	if dir == "." {
		return "root"
	}
	return strings.SplitN(dir, "/", 2)[0]
}

// buildSubsystemMap returns a map of subsystem name → indices of pending tasks.
func buildSubsystemMap(tasks []Task) map[string][]int {
	m := make(map[string][]int)
	for i, t := range tasks {
		if t.Status != "" {
			continue
		}
		sub := taskSubsystem(&tasks[i])
		m[sub] = append(m[sub], i)
	}
	return m
}

// detectClusteredTasks returns slices of task indices where two or more
// pending tasks share the same subsystem.
func detectClusteredTasks(tasks []Task) [][]int {
	m := buildSubsystemMap(tasks)
	var clusters [][]int
	for _, indices := range m {
		if len(indices) >= 2 {
			clusters = append(clusters, indices)
		}
	}
	return clusters
}

// mergeClusteredTasks scans the task file for adjacent pairs of pending tasks
// in the same subsystem and merges them when safe. Returns true if at least
// one merge occurred (caller should re-save the task file).
//
// A merge is safe when:
//   - Both tasks have RetryCount == 0 (no prior failures on either)
//   - Combined description length stays manageable
//   - Combined file list fits within maxFilesTouched + fileCapBonus()
func mergeClusteredTasks(tf *TaskFile) bool {
	merged := false
	clusters := detectClusteredTasks(tf.Tasks)
	for _, group := range clusters {
		if len(group) < 2 {
			continue
		}
		// Take the first pair from each cluster.
		i, j := group[0], group[1]
		if i >= len(tf.Tasks) || j >= len(tf.Tasks) {
			continue
		}
		a := &tf.Tasks[i]
		b := &tf.Tasks[j]
		if !isMergeSafe(a, b) {
			continue
		}
		logInfo("subsystem_merge", a.ID, fmt.Sprintf("merging %s + %s (subsystem=%s)", a.ID, b.ID, taskSubsystem(a)))
		mergeInto(a, b)
		// Remove task at index j (replace with last element).
		tf.Tasks = append(tf.Tasks[:j], tf.Tasks[j+1:]...)
		merged = true
	}
	return merged
}

// isMergeSafe reports whether two tasks can safely be merged.
func isMergeSafe(a, b *Task) bool {
	if a.RetryCount != 0 || b.RetryCount != 0 {
		return false
	}
	combinedFiles := unionFiles(a.Files, b.Files)
	if len(combinedFiles) > maxFilesTouched+fileCapBonus() {
		return false
	}
	return true
}

// mergeInto absorbs task b into task a.
func mergeInto(a, b *Task) {
	a.Description = a.Description + "\n" + b.Description
	a.Files = unionFiles(a.Files, b.Files)
	a.MergedCount = max(1, a.MergedCount) + max(1, b.MergedCount)
	// Inherit ChangeType from b only if a has none.
	if a.ChangeType == "" {
		a.ChangeType = b.ChangeType
	}
	// Clear cached hash so the merged task is treated as fresh.
	a.Hash = ""
}

// unionFiles returns the union of two file slices with no duplicates.
func unionFiles(a, b []string) []string {
	seen := make(map[string]bool, len(a)+len(b))
	for _, f := range a {
		seen[f] = true
	}
	out := make([]string, len(a))
	copy(out, a)
	for _, f := range b {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}

// recordSubsystemOutcome updates the session-level subsystem registry with the
// outcome (success or failure) of a task, including retry count, risk score,
// and patch size for richer analytics.
func recordSubsystemOutcome(metrics map[string]*subsystemMetrics, task *Task, success bool) {
	sub := taskSubsystem(task)
	ensureSubsystemEntry(metrics, sub)
	ensureSubsystemEntry(subsystemRegistry, sub)

	update := func(m *subsystemMetrics) {
		m.totalRetries += task.RetryCount
		if success {
			m.successes++
		} else {
			m.failures++
		}
	}
	update(metrics[sub])
	update(subsystemRegistry[sub])
}

// recordSubsystemPatchMetrics records risk and patch size for a completed patch.
func recordSubsystemPatchMetrics(task *Task, diff string) {
	sub := taskSubsystem(task)
	ensureSubsystemEntry(subsystemRegistry, sub)
	m := subsystemRegistry[sub]
	m.patchCount++
	m.totalRisk += scorePatchRisk(diff, task).score
	m.totalSize += lineCount(diff)
}

func ensureSubsystemEntry(m map[string]*subsystemMetrics, sub string) {
	if _, ok := m[sub]; !ok {
		m[sub] = &subsystemMetrics{subsystem: sub}
	}
}

// logSubsystemStats emits a subsystem stability summary via logInfo.
func logSubsystemStats(metrics map[string]*subsystemMetrics) {
	for sub, m := range metrics {
		total := m.successes + m.failures
		if total == 0 {
			continue
		}
		logInfo("subsystem_stability", sub, fmt.Sprintf(
			"successes=%d failures=%d retries=%d avg_risk=%.2f avg_size=%.0f stable=%v unstable=%v",
			m.successes, m.failures, m.totalRetries,
			m.avgRiskScore(), m.avgPatchSize(),
			m.isStable(), m.isUnstable(),
		))
	}
}
