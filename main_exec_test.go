package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordRetryConvergenceTracksRepeatedFailures(t *testing.T) {
	stats := executionStats{}
	stats.recordRetryConvergence("R1", 1, "undefined symbol", "undefined symbol")
	if stats.convergenceSamples != 0 || stats.convergenceAlerts != 0 {
		t.Fatalf("unexpected counts before threshold: %#v", stats)
	}

	stats.recordRetryConvergence("R1", 2, "undefined symbol", "undefined symbol")
	if stats.convergenceSamples != 1 || stats.convergenceAlerts != 1 {
		t.Fatalf("expected one alert, got samples=%d alerts=%d", stats.convergenceSamples, stats.convergenceAlerts)
	}

	stats.recordRetryConvergence("R1", 3, "undefined symbol", "type mismatch")
	if stats.convergenceSamples != 2 || stats.convergenceAlerts != 1 {
		t.Fatalf("expected convergence sample without alert, got samples=%d alerts=%d", stats.convergenceSamples, stats.convergenceAlerts)
	}
}

func TestRevertBuildFailurePatchesRevertsFixesAndOriginal(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	path := filepath.Join(tmpDir, "sample.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	originalDiff := strings.Join([]string{
		"diff --git a/sample.txt b/sample.txt",
		"--- a/sample.txt",
		"+++ b/sample.txt",
		"@@ -1 +1 @@",
		"-a",
		"+b",
	}, "\n") + "\n"
	fixDiff := strings.Join([]string{
		"diff --git a/sample.txt b/sample.txt",
		"--- a/sample.txt",
		"+++ b/sample.txt",
		"@@ -1 +1 @@",
		"-b",
		"+c",
	}, "\n") + "\n"

	if err := applyPatch(originalDiff); err != nil {
		t.Fatalf("apply original diff: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after original patch: %v", err)
	}
	if got := string(data); got != "b\n" {
		t.Fatalf("expected original patch to update contents to b, got %q", got)
	}

	if err := applyPatch(fixDiff); err != nil {
		t.Fatalf("apply fix diff: %v", err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after fix patch: %v", err)
	}
	if got := string(data); got != "c\n" {
		t.Fatalf("expected fix patch to update contents to c, got %q", got)
	}

	if err := revertBuildFailurePatches(originalDiff, []string{fixDiff}, nil); err != nil {
		t.Fatalf("revert build failure patches: %v", err)
	}

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got := string(data); got != "a\n" {
		t.Fatalf("expected file restored to original contents, got %q", got)
	}
}

func TestRevertBuildFailurePatchesRestoresTrivialFixSnapshot(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	path := filepath.Join(tmpDir, "sample.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write sample file: %v", err)
	}

	originalDiff := strings.Join([]string{
		"diff --git a/sample.txt b/sample.txt",
		"--- a/sample.txt",
		"+++ b/sample.txt",
		"@@ -1 +1 @@",
		"-a",
		"+b",
	}, "\n") + "\n"
	fixDiff := strings.Join([]string{
		"diff --git a/sample.txt b/sample.txt",
		"--- a/sample.txt",
		"+++ b/sample.txt",
		"@@ -1 +1 @@",
		"-bt",
		"+ct",
	}, "\n") + "\n"

	if err := applyPatch(originalDiff); err != nil {
		t.Fatalf("apply original diff: %v", err)
	}

	if err := os.WriteFile(path, []byte("bt\n"), 0o644); err != nil {
		t.Fatalf("write trivial fix content: %v", err)
	}

	if err := applyPatch(fixDiff); err != nil {
		t.Fatalf("apply fix diff: %v", err)
	}

	snapshots := map[string]fileSnapshot{
		"sample.txt": {
			existed: true,
			mode:    0o644,
			data:    []byte("b\n"),
		},
	}
	if err := revertBuildFailurePatches(originalDiff, []string{fixDiff}, snapshots); err != nil {
		t.Fatalf("revert build failure patches: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if got := string(data); got != "a\n" {
		t.Fatalf("expected file restored to original contents, got %q", got)
	}
}

func TestWorkspaceDirtyDetectsStagedAndUnstaged(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	initGitRepo(t, tmpDir)
	if err := os.WriteFile("sample.txt", []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	runCmd(t, "git", "add", "sample.txt")
	runCmd(t, "git", "commit", "-m", "initial")

	dirty, err := workspaceDirty()
	if err != nil || dirty {
		t.Fatalf("expected clean workspace, dirty=%v err=%v", dirty, err)
	}

	if err := os.WriteFile("sample.txt", []byte("b\n"), 0o644); err != nil {
		t.Fatalf("write unstaged change: %v", err)
	}
	dirty, err = workspaceDirty()
	if err != nil || !dirty {
		t.Fatalf("expected dirty workspace for unstaged changes, dirty=%v err=%v", dirty, err)
	}

	runCmd(t, "git", "checkout", "--", "sample.txt")
	if err := os.WriteFile("sample.txt", []byte("staged\n"), 0o644); err != nil {
		t.Fatalf("write staged change: %v", err)
	}
	runCmd(t, "git", "add", "sample.txt")
	dirty, err = workspaceDirty()
	if err != nil || !dirty {
		t.Fatalf("expected dirty workspace for staged changes, dirty=%v err=%v", dirty, err)
	}
}

func TestEnsureCleanWorkspaceResetsAndPreservesExcludedFiles(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})

	initGitRepo(t, tmpDir)
	if err := os.WriteFile("tracked.txt", []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	if err := os.WriteFile("tasks.json", []byte(`{"tasks":[]}`), 0o644); err != nil {
		t.Fatalf("write tasks file: %v", err)
	}
	if err := os.WriteFile("orchestrator.log", []byte("log"), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}
	if err := os.WriteFile("orchestrator-journal.json", []byte("{}"), 0o644); err != nil {
		t.Fatalf("write journal file: %v", err)
	}
	runCmd(t, "git", "add", "tracked.txt")
	runCmd(t, "git", "commit", "-m", "initial")

	if err := os.WriteFile("tracked.txt", []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty tracked file: %v", err)
	}
	if err := os.WriteFile("temp.txt", []byte("remove me"), 0o644); err != nil {
		t.Fatalf("write untracked file: %v", err)
	}

	prevDryRun, prevSkip := dryRun, skipWorkspaceReset
	dryRun, skipWorkspaceReset = false, false
	t.Cleanup(func() {
		dryRun, skipWorkspaceReset = prevDryRun, prevSkip
	})

	if err := ensureCleanWorkspace("T1"); err != nil {
		t.Fatalf("ensureCleanWorkspace: %v", err)
	}

	data, err := os.ReadFile("tracked.txt")
	if err != nil {
		t.Fatalf("read tracked file: %v", err)
	}
	if string(data) != "a\n" {
		t.Fatalf("expected tracked file reset, got %q", data)
	}
	if _, err := os.Stat("temp.txt"); !os.IsNotExist(err) {
		t.Fatalf("expected temp.txt removed, stat err=%v", err)
	}
	for _, keep := range []string{"tasks.json", "orchestrator.log", "orchestrator-journal.json"} {
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("expected %s preserved: %v", keep, err)
		}
	}
}
