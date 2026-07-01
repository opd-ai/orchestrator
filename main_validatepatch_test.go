package main

import (
	"strings"
	"testing"
)

func TestValidatePatchRejectsUnexpectedFiles(t *testing.T) {
	maxPatchLines = 50
	maxFilesTouched = 3

	task := &Task{
		Description: "Update planner",
		Files:       []string{"main.go"},
	}

	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1 @@",
		"-old",
		"+new",
		"diff --git a/main_exec.go b/main_exec.go",
		"--- a/main_exec.go",
		"+++ b/main_exec.go",
		"@@ -1 +1 @@",
		"-old",
		"+new",
	}, "\n")

	err := validatePatch(diff, task.Files, task)
	if err == nil || !strings.Contains(err.Error(), "outside the allowed set") {
		t.Fatalf("expected allowed-file rejection, got %v", err)
	}
}

func TestValidatePatchAllowsResolvedContextWithoutExplicitFiles(t *testing.T) {
	maxPatchLines = 50
	maxFilesTouched = 3

	task := &Task{Description: "Update planner"}
	diff := strings.Join([]string{
		"diff --git a/main_exec.go b/main_exec.go",
		"--- a/main_exec.go",
		"+++ b/main_exec.go",
		"@@ -1 +1,3 @@",
		"-old",
		"+new",
		"+extra",
		"+more",
	}, "\n")

	if err := validatePatch(diff, []string{"main.go"}, task); err != nil {
		t.Fatalf("expected context-only file list to pass, got %v", err)
	}
}

func TestValidatePatchRejectsDeletionHeavyDiff(t *testing.T) {
	maxPatchLines = 50
	maxFilesTouched = 3

	task := &Task{Description: "Update planner"}
	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,3 +1 @@",
		"-line1",
		"-line2",
		"+line3",
	}, "\n")

	err := validatePatch(diff, nil, task)
	if err == nil || !strings.Contains(err.Error(), "30%") {
		t.Fatalf("expected deletion-ratio rejection, got %v", err)
	}
}
