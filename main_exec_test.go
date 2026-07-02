package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordRetryConvergenceTracksRepeatedFailures(t *testing.T) {
	stats := executionStats{}
	if forced := stats.recordRetryConvergence("R1", 1, "undefined symbol", "undefined symbol"); forced {
		t.Fatal("retry 1 should not force architect mode")
	}
	if stats.convergenceSamples != 0 || stats.convergenceAlerts != 0 {
		t.Fatalf("unexpected counts before threshold: %#v", stats)
	}

	if forced := stats.recordRetryConvergence("R1", 2, "undefined symbol", "undefined symbol"); !forced {
		t.Fatal("retry 2 should force architect mode after repeated failure")
	}
	if stats.convergenceSamples != 1 || stats.convergenceAlerts != 1 {
		t.Fatalf("expected one alert, got samples=%d alerts=%d", stats.convergenceSamples, stats.convergenceAlerts)
	}

	if forced := stats.recordRetryConvergence("R1", 3, "undefined symbol", "type mismatch"); forced {
		t.Fatal("changed failure category should not force architect mode")
	}
	if stats.convergenceSamples != 2 || stats.convergenceAlerts != 1 {
		t.Fatalf("expected convergence sample without alert, got samples=%d alerts=%d", stats.convergenceSamples, stats.convergenceAlerts)
	}
}

func TestFixRetrySettings(t *testing.T) {
	prevArchitect, prevExecutor, prevModel, prevEscalation := architectModelName, executorModelName, modelName, activeModelEscalation
	architectModelName = "architect-large"
	executorModelName = "executor-small"
	modelName = "default-model"
	activeModelEscalation = modelEscalationState{}
	t.Cleanup(func() {
		architectModelName = prevArchitect
		executorModelName = prevExecutor
		modelName = prevModel
		activeModelEscalation = prevEscalation
	})

	temp, model := fixRetrySettings(2, false)
	if temp != 0.7 || model != "executor-small" {
		t.Fatalf("normal retry = (%v, %q), want (0.7, %q)", temp, model, "executor-small")
	}

	temp, model = fixRetrySettings(3, true)
	if temp != 0.8 || model != "architect-large" {
		t.Fatalf("forced architect retry = (%v, %q), want (0.8, %q)", temp, model, "architect-large")
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

func TestEnsureCleanWorkspaceResetsStagedChanges(t *testing.T) {
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
	if err := os.WriteFile("staged.txt", []byte("original\n"), 0o644); err != nil {
		t.Fatalf("write staged file: %v", err)
	}
	runCmd(t, "git", "add", "staged.txt")
	runCmd(t, "git", "commit", "-m", "initial")

	if err := os.WriteFile("staged.txt", []byte("modified\n"), 0o644); err != nil {
		t.Fatalf("write modified file: %v", err)
	}
	runCmd(t, "git", "add", "staged.txt")

	dirty, err := workspaceDirty()
	if err != nil || !dirty {
		t.Fatalf("expected dirty workspace for staged changes, dirty=%v err=%v", dirty, err)
	}

	prevDryRun, prevSkip := dryRun, skipWorkspaceReset
	dryRun, skipWorkspaceReset = false, false
	t.Cleanup(func() {
		dryRun, skipWorkspaceReset = prevDryRun, prevSkip
	})

	if err := ensureCleanWorkspace("T1"); err != nil {
		t.Fatalf("ensureCleanWorkspace: %v", err)
	}

	data, err := os.ReadFile("staged.txt")
	if err != nil {
		t.Fatalf("read staged file: %v", err)
	}
	if string(data) != "original\n" {
		t.Fatalf("expected staged file reset to original, got %q", data)
	}

	dirty, err = workspaceDirty()
	if err != nil || dirty {
		t.Fatalf("expected clean workspace after reset, dirty=%v err=%v", dirty, err)
	}
}

func TestGatherAndValidateDiffPersistsRetryCountOnTooLargePatch(t *testing.T) {
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

	prevMaxPatchLines, prevMaxRetries, prevMaxFilesTouched := maxPatchLines, maxRetries, maxFilesTouched
	maxPatchLines, maxRetries, maxFilesTouched = 1, 3, 3
	t.Cleanup(func() {
		maxPatchLines, maxRetries, maxFilesTouched = prevMaxPatchLines, prevMaxRetries, prevMaxFilesTouched
	})

	task := Task{
		ID:          "T1",
		Description: "update target file",
		Status:      "in_progress",
		Files:       []string{"target.go"},
		Hash:        "task-hash",
	}
	if err := saveTasks(TaskFile{Tasks: []Task{task}}); err != nil {
		t.Fatalf("saveTasks() error = %v", err)
	}

	diffLines := []string{
		"diff --git a/target.go b/target.go",
		"--- a/target.go",
		"+++ b/target.go",
		"@@ -0,0 +1,11 @@",
	}
	for i := 1; i <= 11; i++ {
		diffLines = append(diffLines, "+line"+strings.Repeat("x", i))
	}
	diff := strings.Join(diffLines, "\n") + "\n"
	taskCache := map[string]string{
		task.Hash: diff,
	}

	stats := newExecutionStats()
	tf := loadTasks()
	_, _, _, ok := gatherAndValidateDiff(&tf, &tf.Tasks[0], taskCache, &stats)
	if ok {
		t.Fatalf("expected oversized patch validation to fail")
	}

	saved := loadTasks()
	if got := saved.Tasks[0].RetryCount; got != 1 {
		t.Fatalf("expected persisted retry count of 1, got %d", got)
	}
	if got := stats.totalRetries; got != 1 {
		t.Fatalf("expected stats totalRetries to be 1, got %d", got)
	}
}
