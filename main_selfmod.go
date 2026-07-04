package main

import (
	"fmt"
	"strings"
)

// protectedFiles is the canonical manifest of core runtime files whose mutation
// requires heightened validation. These files form the execution loop, task
// scheduler, branch/journal recovery, and memory persistence code paths.
// Any change to this list must be accompanied by an update to the corresponding
// test in main_selfmod_test.go.
var protectedFiles = []string{
	"main.go",
	"main_exec.go",
	"main_helper.go",
	"main_journal.go",
	"main_tasks.go",
	"main_selfmod.go",
	"memory/metrics.go",
	"memory/runlog.go",
	"memory/branch.go",
}

// touchesProtectedFile reports whether the unified diff modifies at least one
// file listed in protectedFiles. The comparison is case-sensitive and matches
// on exact base names produced by filesTouched.
func touchesProtectedFile(diff string) bool {
	touched := filesTouched(diff)
	protected := protectedFilesSet()
	for _, f := range touched {
		if protected[f] {
			return true
		}
	}
	return false
}

// protectedFilesSet returns protectedFiles as a lookup map for O(1) membership
// tests.
func protectedFilesSet() map[string]bool {
	set := make(map[string]bool, len(protectedFiles))
	for _, f := range protectedFiles {
		set[f] = true
	}
	return set
}

// logSelfEditAttempt emits a structured log entry with a truncated diff preview
// before a patch that touches protected files is applied to the workspace.
// The preview is capped at selfEditPreviewBytes to keep log entries readable.
func logSelfEditAttempt(taskID, diff string) {
	preview := diff
	if len(preview) > selfEditPreviewBytes {
		preview = preview[:selfEditPreviewBytes] + "\n... (truncated)"
	}
	logInfo("self_edit_attempt", taskID, fmt.Sprintf("touches_protected_files=true diff_preview=%q", preview))
}

// logSelfEditOutcome records the result of a protected-file edit attempt.
// success=true means the build passed and the patch was committed; success=false
// means validation failed and the workspace was reverted.
func logSelfEditOutcome(taskID string, success bool, buildErr string) {
	if success {
		logInfo("self_edit_committed", taskID, "protected_file_patch_passed_validation")
	} else {
		detail := buildErr
		if detail == "" {
			detail = "validation_failed"
		}
		logError("self_edit_reverted", taskID, fmt.Sprintf("protected_file_patch_failed: %s",
			strings.TrimSpace(detail)))
	}
}

// maybeLogSelfEditAttempt emits a self_edit_attempt log entry when the diff
// touches a protected file. It is a no-op for ordinary task patches.
func maybeLogSelfEditAttempt(taskID, diff string) {
	if touchesProtectedFile(diff) {
		logSelfEditAttempt(taskID, diff)
	}
}

// maybeLogSelfEditOutcome emits a self_edit_committed or self_edit_reverted log
// entry when the diff touches a protected file. buildOut is the compiler output
// on failure, or empty string on success.
func maybeLogSelfEditOutcome(taskID, diff, buildOut string) {
	if touchesProtectedFile(diff) {
		logSelfEditOutcome(taskID, buildOut == "", buildOut)
	}
}

// selfEditPreviewBytes is the maximum number of bytes of a protected-file diff
// included in the self_edit_attempt log entry.
const selfEditPreviewBytes = 512
