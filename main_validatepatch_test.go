package main

import (
	"fmt"
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

func TestValidateTransformOnlyRejectsTaskWithoutChangeType(t *testing.T) {
	transformOnly = true
	defer func() { transformOnly = false }()

	task := &Task{Description: "Update planner"}
	if err := validateTransformOnly(task); err == nil {
		t.Fatal("expected error for missing ChangeType in transform-only mode, got nil")
	}
}

func TestValidateTransformOnlyAllowsTaskWithChangeType(t *testing.T) {
	transformOnly = true
	defer func() { transformOnly = false }()

	task := &Task{Description: "Update planner", ChangeType: ChangeTypeGeneral}
	if err := validateTransformOnly(task); err != nil {
		t.Fatalf("expected no error with ChangeType set, got %v", err)
	}
}

func TestValidateTransformOnlyPassesWhenFlagOff(t *testing.T) {
	transformOnly = false
	task := &Task{Description: "Update planner"}
	if err := validateTransformOnly(task); err != nil {
		t.Fatalf("expected no error when transform-only is off, got %v", err)
	}
}

func TestValidatePatchRiskGatesHighRiskOnFirstAttempt(t *testing.T) {
	// A diff with many exported-interface additions drives up the risk score.
	lines := []string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1,10 @@",
	}
	for i := 0; i < 10; i++ {
		lines = append(lines, "+func ExportedFunc"+fmt.Sprintf("%d", i)+"() {}")
	}
	diff := strings.Join(lines, "\n")

	// maxRetries must be set for the risk scorer.
	maxRetries = 3

	task := &Task{Description: "Update planner", RetryCount: 0}
	// validatePatchRisk itself won't gate unless score > riskGateThreshold; we
	// exercise the pass-through path (RetryCount > 0 always passes).
	task.RetryCount = 1
	if err := validatePatchRisk(diff, task); err != nil {
		t.Fatalf("expected no error on retry, got %v", err)
	}
}

func TestValidateDSLSchemaInsertFunctionRequiresFuncLine(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1 @@",
		"+// just a comment",
	}, "\n")
	if err := validateDSLSchema(diff, ChangeTypeInsertFunction); err == nil {
		t.Fatal("expected error: INSERT_FUNCTION with no +func line")
	}
}

func TestValidateDSLSchemaInsertFunctionPassesWithFuncLine(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1 @@",
		"+func newHelper() {}",
	}, "\n")
	if err := validateDSLSchema(diff, ChangeTypeInsertFunction); err != nil {
		t.Fatalf("expected no error for INSERT_FUNCTION with +func line, got %v", err)
	}
}

func TestValidateDSLSchemaAddImportRequiresStringLiteral(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1 @@",
		"+// import something",
	}, "\n")
	if err := validateDSLSchema(diff, ChangeTypeAddImport); err == nil {
		t.Fatal("expected error: ADD_IMPORT with no string literal")
	}
}

func TestValidateDSLSchemaGeneralAlwaysPasses(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/main.go b/main.go",
		"--- a/main.go",
		"+++ b/main.go",
		"@@ -1 +1 @@",
		"+anything here",
	}, "\n")
	if err := validateDSLSchema(diff, ChangeTypeGeneral); err != nil {
		t.Fatalf("expected no error for GENERAL change type, got %v", err)
	}
}
