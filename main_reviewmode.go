package main

import "fmt"

// strategicReviewBound caps the context expansion factor in review mode.
// Context grows to at most reviewContextFactor × maxContextFiles files.
const reviewContextFactor = 2

// isStrategicReviewActive tracks whether the current task is executing in
// Strategic Review Mode. It is always reset by deescalateTier/deescalateModel
// at the start of each new task.
var isStrategicReviewActive bool

// shouldTriggerStrategicReview returns true when conditions warrant a bounded
// architectural reasoning burst.
//
// Trigger conditions (any one sufficient):
//   - activeTier >= Tier2Architectural AND stability safe mode active
//   - API surface risk factor > 0.50 (task touches exported signatures)
//   - oscillationCount >= 5 (repeated convergence failures)
func shouldTriggerStrategicReview(task *Task, stats *executionStats) bool {
	if isStrategicReviewActive {
		return false // already in review mode for this task
	}
	if activeTier >= Tier2Architectural && stats.stability.SafeMode {
		return true
	}
	if containsAPIKeyword(task.Description) {
		return true
	}
	if stats.stability.oscillationCount >= 5 {
		return true
	}
	return false
}

// containsAPIKeyword returns true when the task description suggests the task
// touches exported interfaces or public API surface.
func containsAPIKeyword(desc string) bool {
	keywords := []string{"interface", "api", "public", "export", "signature", "contract"}
	lower := toLower(desc)
	for _, kw := range keywords {
		if contains(lower, kw) {
			return true
		}
	}
	return false
}

// gatherExpandedContext returns a bounded expansion of the task's context files
// (up to reviewContextFactor × maxContextFiles).
func gatherExpandedContext(task *Task, baseFiles []string) string {
	cap := maxContextFiles * reviewContextFactor
	expanded := resolveExpandedFiles(task, cap)
	return gatherContextForTask(task, expanded)
}

// resolveExpandedFiles resolves up to capFiles context files for the task,
// preferring explicitly listed files and supplementing with keyword matches.
func resolveExpandedFiles(task *Task, capFiles int) []string {
	seen := make(map[string]bool)
	var files []string

	for _, f := range task.Files {
		if !seen[f] {
			seen[f] = true
			files = append(files, f)
		}
	}

	if len(files) >= capFiles {
		return files[:capFiles]
	}

	// Supplement with keyword-matched files from git ls-files.
	kw := keyword(task.Description)
	if kw == "" {
		return files
	}
	for _, f := range allGoFiles() {
		if seen[f] {
			continue
		}
		if contains(toLower(f), kw) {
			seen[f] = true
			files = append(files, f)
		}
		if len(files) >= capFiles {
			break
		}
	}
	return files
}

// executeInReviewMode runs the task using the architect model with expanded
// context. After execution the invariant and build checks in execute() ensure
// correctness before acceptance. Tier is reset to 0 by the normal
// deescalateTier call at the start of the next task iteration.
func executeInReviewMode(task *Task, stats *executionStats) string {
	isStrategicReviewActive = true
	logInfo("strategic_review_started", task.ID, fmt.Sprintf(
		"oscillations=%d safe_mode=%v tier=%d",
		stats.stability.oscillationCount, stats.stability.SafeMode, activeTier,
	))

	expandedContext := gatherExpandedContext(task, resolveContextFiles(task))
	prompt := promptWithMemory(buildExecPrompt(task, expandedContext))
	diff := callLLMWithModel(prompt, 0.4, roleModel(architectModelName))

	logInfo("strategic_review_completed", task.ID, fmt.Sprintf("lines=%d", lineCount(diff)))
	return diff
}

// deescalateReviewMode resets the strategic review flag. Called alongside
// deescalateTier at the start of each task iteration.
func deescalateReviewMode() {
	isStrategicReviewActive = false
}

// toLower is a package-local wrapper to avoid importing strings in this file.
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := range s {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// contains reports whether substr appears in s.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstr(s, substr))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
