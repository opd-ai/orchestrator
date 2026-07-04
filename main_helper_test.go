package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifyActiveBranchMatchesExpected confirms that verifyActiveBranch
// returns nil when the current HEAD matches the given name.
func TestVerifyActiveBranchMatchesExpected(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	initGitRepo(t, tmpDir)
	// git init leaves HEAD on the default branch; create a commit so the branch
	// name is stable, then switch to a known branch.
	if err := os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runCmd(t, "git", "add", ".")
	runCmd(t, "git", "commit", "-m", "init")
	runCmd(t, "git", "checkout", "-b", "autonomous/test-branch")

	if err := verifyActiveBranch("autonomous/test-branch"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// TestVerifyActiveBranchDetectsMismatch confirms that verifyActiveBranch
// returns a descriptive error when the active branch does not match.
func TestVerifyActiveBranchDetectsMismatch(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	initGitRepo(t, tmpDir)
	if err := os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runCmd(t, "git", "add", ".")
	runCmd(t, "git", "commit", "-m", "init")
	runCmd(t, "git", "checkout", "-b", "real-branch")

	err = verifyActiveBranch("expected-branch")
	if err == nil {
		t.Fatal("expected error for branch mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "expected-branch") || !strings.Contains(err.Error(), "real-branch") {
		t.Fatalf("error should mention both branch names, got: %v", err)
	}
}

// TestVerifyNotDefaultBranchPassesOnAutonomousBranch confirms that an
// autonomous/ branch does not trigger the default-branch guard.
func TestVerifyNotDefaultBranchPassesOnAutonomousBranch(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	initGitRepo(t, tmpDir)
	if err := os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runCmd(t, "git", "add", ".")
	runCmd(t, "git", "commit", "-m", "init")
	runCmd(t, "git", "checkout", "-b", "autonomous/12345")

	if err := verifyNotDefaultBranch(); err != nil {
		t.Fatalf("expected no error on autonomous branch, got %v", err)
	}
}

// TestVerifyNotDefaultBranchRejectsMainBranch confirms the guard fires on "main".
func TestVerifyNotDefaultBranchRejectsMainBranch(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	// Initialise with "main" as the default branch name.
	runCmd(t, "git", "init", "-b", "main", tmpDir)
	runCmd(t, "git", "-C", tmpDir, "config", "user.email", "test@example.com")
	runCmd(t, "git", "-C", tmpDir, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runCmd(t, "git", "add", ".")
	runCmd(t, "git", "commit", "-m", "init")

	verifyErr := verifyNotDefaultBranch()
	if verifyErr == nil {
		t.Fatal("expected error for main branch, got nil")
	}
	if !strings.Contains(verifyErr.Error(), "main") {
		t.Fatalf("error should mention branch name, got: %v", verifyErr)
	}
}

// TestCreateBranchVerifiesActiveAfterCheckout confirms that createBranch sets
// the active branch to the newly created one.
func TestCreateBranchVerifiesActiveAfterCheckout(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	initGitRepo(t, tmpDir)
	if err := os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runCmd(t, "git", "add", ".")
	runCmd(t, "git", "commit", "-m", "init")

	if err := createBranch("autonomous/test"); err != nil {
		t.Fatalf("createBranch: %v", err)
	}
	if got := currentGitBranch(); got != "autonomous/test" {
		t.Fatalf("expected branch autonomous/test, got %q", got)
	}
}

// TestCreateBranchFailsWhenCheckoutFails confirms that createBranch propagates
// errors when the underlying git checkout fails (e.g. branch already exists).
func TestCreateBranchFailsWhenCheckoutFails(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	initGitRepo(t, tmpDir)
	if err := os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runCmd(t, "git", "add", ".")
	runCmd(t, "git", "commit", "-m", "init")
	runCmd(t, "git", "checkout", "-b", "dup-branch")

	// Creating the same branch a second time must fail.
	err = createBranch("dup-branch")
	if err == nil {
		t.Fatal("expected error when creating duplicate branch, got nil")
	}
}
