package main

import (
	"os"
	"strings"
	"testing"
)

func TestEnforceTaskGranularitySplitsMultiFileTask(t *testing.T) {
	tf := TaskFile{
		Tasks: []Task{
			{ID: "R1", Description: "Update validation", Files: []string{"a.go", "b.go"}, Status: "pending"},
		},
	}

	task := &tf.Tasks[0]
	if !enforceTaskGranularity(&tf, task) {
		t.Fatal("expected multi-file task to split")
	}
	if len(tf.Tasks) != 2 {
		t.Fatalf("expected 2 subtasks, got %d", len(tf.Tasks))
	}
	if len(tf.Tasks[0].Files) != 1 || len(tf.Tasks[1].Files) != 1 {
		t.Fatalf("expected each subtask to target one file, got %+v", tf.Tasks)
	}
}

func TestEnforceTaskGranularitySplitsOversizedTask(t *testing.T) {
	desc := "Update retry logic and tighten patch validation and improve error hint formatting"
	tf := TaskFile{
		Tasks: []Task{
			// Empty Files ensures the symbol-split path is skipped and description-based splitting is tested.
			{ID: "R2", Description: desc, Status: "pending"},
		},
	}

	task := &tf.Tasks[0]
	if !enforceTaskGranularity(&tf, task) {
		t.Fatal("expected oversized task to split")
	}
	if len(tf.Tasks) < 2 {
		t.Fatalf("expected split subtasks, got %d", len(tf.Tasks))
	}
}

func TestEnforceTaskGranularitySymbolSplit(t *testing.T) {
	// Write a temp Go file with multiple symbols so symbolTasksForFiles returns ≥2 tasks.
	dir := t.TempDir()
	goFile := dir + "/sample.go"
	content := `package sample

func Alpha() {}
func Beta() {}
func Gamma() {}
`
	if err := os.WriteFile(goFile, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	tf := TaskFile{
		Tasks: []Task{
			{ID: "T1", Description: "refactor sample.go", Files: []string{goFile}, Status: "pending"},
		},
	}
	task := &tf.Tasks[0]
	split := enforceTaskGranularity(&tf, task)
	if !split {
		t.Fatal("expected enforceTaskGranularity to split single-file task with many symbols")
	}
	if len(tf.Tasks) < 2 {
		t.Fatalf("expected ≥2 symbol subtasks, got %d: %+v", len(tf.Tasks), tf.Tasks)
	}
	for _, sub := range tf.Tasks {
		if !symbolTaskRe.MatchString(sub.ID) {
			t.Errorf("symbol subtask ID should match .s<digits>, got %q", sub.ID)
		}
		if len(sub.Files) != 1 {
			t.Errorf("symbol subtask should target one file, got %v", sub.Files)
		}
	}
}

func TestIsAlreadySymbolTask(t *testing.T) {
	if !isAlreadySymbolTask("T1.s3") {
		t.Error("T1.s3 should be recognised as a symbol task")
	}
	if isAlreadySymbolTask("T1.1") {
		t.Error("T1.1 should not be recognised as a symbol task")
	}
}

func TestExecutionBlockFormat(t *testing.T) {
	maxPatchLines = 50
	task := &Task{ID: "R3", Files: []string{"main.go"}}
	block := executionBlock("FIX", task, []string{"a", "b"}, "compiler error summary")
	for _, want := range []string{
		"MODE: FIX",
		"TASK_ID: R3",
		"FILES_ALLOWED: main.go",
		"MAX_PATCH_LINES: 50",
		"MAX_FILE_PATCH_LINES: 50",
		"CONSTRAINTS:",
		"FAIL_REASON:",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("executionBlock() missing %q in %q", want, block)
		}
	}
}

func TestBuildFixPromptIncludesPreviousAttemptPreview(t *testing.T) {
	lines := make([]string, 0, 25)
	for i := 1; i <= 25; i++ {
		lines = append(lines, strings.Repeat("x", i))
	}
	prompt := buildFixPrompt(
		&Task{ID: "R4", Description: "Fix build"},
		"context",
		fixTaskConfig{
			hints:        "compiler error",
			previousDiff: strings.Join(lines, "\n"),
		},
	)
	if !strings.Contains(prompt, "PREVIOUS_ATTEMPT (failed):") {
		t.Fatalf("expected previous attempt block in %q", prompt)
	}
	if !strings.Contains(prompt, lines[19]) {
		t.Fatalf("expected twentieth line preview in %q", prompt)
	}
	if strings.Contains(prompt, lines[20]) {
		t.Fatalf("expected preview to stop at 20 lines, got %q", prompt)
	}
}

func TestTempForRetry(t *testing.T) {
	tests := []struct {
		retry int
		want  float64
	}{
		{retry: 1, want: 0.3},
		{retry: 2, want: 0.7},
		{retry: 3, want: 0.5},
		{retry: 5, want: 0.5},
	}
	for _, tt := range tests {
		if got := tempForRetry(tt.retry); got != tt.want {
			t.Fatalf("tempForRetry(%d) = %v, want %v", tt.retry, got, tt.want)
		}
	}
}
