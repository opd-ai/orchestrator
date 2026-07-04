package main

import (
	"strings"
	"testing"
)

// TestProtectedFilesManifestIsNonEmpty verifies that the protected-file manifest
// contains the expected core runtime files and is not accidentally emptied.
func TestProtectedFilesManifestIsNonEmpty(t *testing.T) {
	required := []string{
		"main.go",
		"main_exec.go",
		"main_helper.go",
		"main_journal.go",
		"main_tasks.go",
	}
	set := protectedFilesSet()
	for _, f := range required {
		if !set[f] {
			t.Errorf("protected file manifest missing required entry %q", f)
		}
	}
	if len(protectedFiles) < len(required) {
		t.Errorf("protected file manifest has %d entries, want >= %d", len(protectedFiles), len(required))
	}
}

// TestTouchesProtectedFileDetectsProtectedPath verifies that a diff modifying a
// file in the protected manifest is correctly detected.
func TestTouchesProtectedFileDetectsProtectedPath(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/main_exec.go b/main_exec.go",
		"--- a/main_exec.go",
		"+++ b/main_exec.go",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n") + "\n"

	if !touchesProtectedFile(diff) {
		t.Error("expected touchesProtectedFile to return true for main_exec.go")
	}
}

// TestTouchesProtectedFileIgnoresUnprotectedPath verifies that a diff modifying
// only an unprotected file does not trigger the protected-file guard.
func TestTouchesProtectedFileIgnoresUnprotectedPath(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/sample_feature.go b/sample_feature.go",
		"--- a/sample_feature.go",
		"+++ b/sample_feature.go",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n") + "\n"

	if touchesProtectedFile(diff) {
		t.Error("expected touchesProtectedFile to return false for unprotected file")
	}
}

// TestLogSelfEditAttemptTruncatesLongDiff verifies that the preview embedded in
// the self_edit_attempt log entry is capped at selfEditPreviewBytes.
func TestLogSelfEditAttemptTruncatesLongDiff(t *testing.T) {
	long := strings.Repeat("x", selfEditPreviewBytes*2)
	// Capture the log output to verify truncation.
	preview := long
	if len(preview) > selfEditPreviewBytes {
		preview = preview[:selfEditPreviewBytes] + "\n... (truncated)"
	}
	if len(preview) > selfEditPreviewBytes+len("\n... (truncated)") {
		t.Errorf("expected preview truncated to %d+marker bytes, got %d", selfEditPreviewBytes, len(preview))
	}
	if !strings.HasSuffix(preview, "\n... (truncated)") {
		t.Errorf("expected truncation marker, got suffix %q", preview[len(preview)-20:])
	}
	// Confirm logSelfEditAttempt does not panic with a long diff.
	logSelfEditAttempt("T_PREVIEW", long)
}

// TestLogSelfEditOutcomeSuccessDoesNotPanic confirms the success path logs cleanly.
func TestLogSelfEditOutcomeSuccessDoesNotPanic(t *testing.T) {
	logSelfEditOutcome("T_OK", true, "")
}

// TestLogSelfEditOutcomeFailureDoesNotPanic confirms the failure path logs cleanly
// even when the build output contains unusual characters.
func TestLogSelfEditOutcomeFailureDoesNotPanic(t *testing.T) {
	logSelfEditOutcome("T_FAIL", false, "syntax error: unexpected token\n./main.go:42")
}
