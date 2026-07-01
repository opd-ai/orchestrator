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

func TestValidatePatchRejectsRename(t *testing.T) {
	maxPatchLines = 50
	maxFilesTouched = 3

	task := &Task{Description: "Update planner"}
	diff := strings.Join([]string{
		"diff --git a/main.go b/main_new.go",
		"rename from main.go",
		"rename to main_new.go",
	}, "\n")

	err := validatePatch(diff, nil, task)
	if err == nil || !strings.Contains(err.Error(), "unexpected rename") {
		t.Fatalf("expected rename rejection, got %v", err)
	}
}

func TestValidatePatchRejectsFullRewrite(t *testing.T) {
	maxPatchLines = 50
	maxFilesTouched = 3

	task := &Task{Description: "Update planner"}
	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1,10 +1,10 @@",
		"-l1", "-l2", "-l3", "-l4", "-l5", "-l6", "-l7", "-l8", "-l9", "-l10",
		"+n1", "+n2", "+n3", "+n4", "+n5", "+n6", "+n7", "+n8", "+n9", "+n10",
	}, "\n")

	err := validatePatch(diff, nil, task)
	if err == nil || !strings.Contains(err.Error(), "rewrite a full file") {
		t.Fatalf("expected full rewrite rejection, got %v", err)
	}
}

func TestValidatePatchRejectsLineDeltaCapPerFile(t *testing.T) {
	maxPatchLines = 10
	maxFilesTouched = 3

	task := &Task{Description: "Update planner"}
	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1,5 @@",
		"-old1",
		"+new1",
		"+new2",
		"+new3",
		"+new4",
		"+new5",
	}, "\n")

	err := validatePatch(diff, nil, task)
	if err == nil || !strings.Contains(err.Error(), "line delta cap") {
		t.Fatalf("expected line delta cap rejection, got %v", err)
	}
}
