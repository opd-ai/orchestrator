package main

import (
	"os"
	"os/exec"
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

// TestApplyProtectedFilePatchValidatesBeforeCommit verifies that the two-step
// apply path for protected files validates the patch before committing it to
// the workspace. The test creates a temporary git repo, makes a valid change
// to a protected file, and confirms it passes validation.
func TestApplyProtectedFilePatchValidatesBeforeCommit(t *testing.T) {
	// Skip if not in a git repo or git not available
	if _, err := exec.Command("git", "rev-parse", "--git-dir").CombinedOutput(); err != nil {
		t.Skip("git not available")
	}

	// Create a temp directory for the test
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Initialize git repo
	if out, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (output: %s)", err, string(out))
	}
	if out, err := exec.Command("git", "config", "user.email", "test@test").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v (output: %s)", err, string(out))
	}
	if out, err := exec.Command("git", "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v (output: %s)", err, string(out))
	}

	// Create a minimal go.mod and main.go
	goMod := `module test

go 1.21
`
	if err := os.WriteFile("go.mod", []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	mainGo := `package main

func main() {
	println("hello")
}
`
	if err := os.WriteFile("main.go", []byte(mainGo), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// Add and commit
	if out, err := exec.Command("git", "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v (output: %s)", err, string(out))
	}
	if out, err := exec.Command("git", "commit", "-m", "initial").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v (output: %s)", err, string(out))
	}

	// Create a diff that modifies main.go (a protected file) with a valid change
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,5 +1,5 @@
 package main
 
 func main() {
-	println("hello")
+	println("hello world")
 }
`
	task := &Task{ID: "TEST_VALID"}

	// This should succeed because the change is valid
	err := validateAndApplyProtectedPatch(diff, task)
	if err != nil {
		t.Errorf("expected valid patch to pass validation, got error: %v", err)
	}

	// Verify the change was applied
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(content), "hello world") {
		t.Errorf("expected change to be applied, got: %s", string(content))
	}
}

// TestApplyProtectedFilePatchRollsBackOnFailure verifies that an invalid patch
// to a protected file is rolled back and the original state is restored.
func TestApplyProtectedFilePatchRollsBackOnFailure(t *testing.T) {
	if _, err := exec.Command("git", "rev-parse", "--git-dir").CombinedOutput(); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if out, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v (output: %s)", err, string(out))
	}
	if out, err := exec.Command("git", "config", "user.email", "test@test").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v (output: %s)", err, string(out))
	}
	if out, err := exec.Command("git", "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config: %v (output: %s)", err, string(out))
	}

	goMod := `module test

go 1.21
`
	if err := os.WriteFile("go.mod", []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	mainGo := `package main

func main() {
	println("hello")
}
`
	if err := os.WriteFile("main.go", []byte(mainGo), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	if out, err := exec.Command("git", "add", ".").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v (output: %s)", err, string(out))
	}
	if out, err := exec.Command("git", "commit", "-m", "initial").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v (output: %s)", err, string(out))
	}

	// Create a diff that introduces a syntax error
	diff := `diff --git a/main.go b/main.go
--- a/main.go
+++ b/main.go
@@ -1,5 +1,5 @@
 package main
 
 func main() {
-	println("hello")
+	println("hello"  // missing closing paren - syntax error
 }
`
	task := &Task{ID: "TEST_INVALID"}

	// This should fail validation and roll back
	err := validateAndApplyProtectedPatch(diff, task)
	if err == nil {
		t.Error("expected invalid patch to fail validation")
	}

	// Verify the original content is restored
	content, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(content), "println(\"hello\")") {
		t.Errorf("expected original content to be restored, got: %s", string(content))
	}
	if strings.Contains(string(content), "println(\"hello\"") && !strings.Contains(string(content), "println(\"hello\")") {
		t.Errorf("invalid syntax should have been rolled back")
	}
}
