package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opd-ai/orchestrator/memory"
)

func TestWriteBuildFailurePersistsPerAttemptArtifacts(t *testing.T) {
	tempDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	writeBuildFailure("R4.1", "compile error one", 0)
	writeBuildFailure("R4.1", "compile error two", 1)

	dir := filepath.Join("logs", "build_failures")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(entries))
	}

	var sawAttempt1, sawAttempt2 bool
	for _, entry := range entries {
		name := entry.Name()
		if strings.Contains(name, "attempt-1") {
			sawAttempt1 = true
		}
		if strings.Contains(name, "attempt-2") {
			sawAttempt2 = true
		}

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read artifact %s: %v", name, err)
		}
		var artifact buildFailureArtifact
		if err := json.Unmarshal(data, &artifact); err != nil {
			t.Fatalf("unmarshal artifact %s: %v", name, err)
		}
		if artifact.TaskID != "R4.1" {
			t.Fatalf("unexpected task id in %s: %q", name, artifact.TaskID)
		}
	}
	if !sawAttempt1 || !sawAttempt2 {
		t.Fatalf("expected attempt-1 and attempt-2 artifacts, got %#v", entries)
	}
}

func TestBuildFailureArtifactNameSanitizesTaskID(t *testing.T) {
	name := buildFailureArtifactName("R4/1 bad task", 0)
	if strings.Contains(name, "/") {
		t.Fatalf("artifact name contains path separator: %q", name)
	}
	if !strings.HasPrefix(name, "R4_1_bad_task-attempt-1-") {
		t.Fatalf("unexpected artifact prefix: %q", name)
	}
}

func TestWriteRejectedPatchPersistsDistinctArtifacts(t *testing.T) {
	tempDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	writeRejectedPatch("R4.2", "diff one")
	writeRejectedPatch("R4.2", "diff two")

	dir := filepath.Join("logs", "rejected_patches")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir %s: %v", dir, err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(entries))
	}
}

func TestWriteRunSummaryIncludesBlockedReasons(t *testing.T) {
	tempDir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})

	writeRunSummary(memory.RunSummary{
		TasksTotal:              3,
		TasksCompleted:          2,
		TasksBlocked:            1,
		DurationSeconds:         12,
		Branch:                  "autonomous/test",
		RetryConvergenceAlerts:  1,
		RetryConvergenceSamples: 2,
		BlockedTaskReasons: map[string]int{
			"patch_rejected":       1,
			"workspace_reset_fail": 2,
		},
	})

	data, err := os.ReadFile("AUTONOMOUS_RUN_SUMMARY.md")
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "## Blocked task reasons") {
		t.Fatalf("expected blocked reasons section in summary: %q", text)
	}
	retryLinePos := strings.Index(text, "Retry convergence alerts")
	reasonHeaderPos := strings.Index(text, "## Blocked task reasons")
	if retryLinePos == -1 || reasonHeaderPos == -1 {
		t.Fatalf("missing retry line or blocked reasons section: %q", text)
	}
	if retryLinePos > reasonHeaderPos {
		t.Fatalf("expected blocked reasons section after retry convergence line: %q", text)
	}
	for _, want := range []string{"- patch_rejected: 1", "- workspace_reset_fail: 2"} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected %q in summary: %q", want, text)
		}
	}
}
