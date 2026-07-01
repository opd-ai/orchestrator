package main

import (
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

func TestExecutionBlockFormat(t *testing.T) {
	maxPatchLines = 50
	task := &Task{ID: "R3", Files: []string{"main.go"}}
	block := executionBlock("FIX", task, []string{"a", "b"}, "compiler error summary")
	for _, want := range []string{
		"MODE: FIX",
		"TASK_ID: R3",
		"FILES_ALLOWED: main.go",
		"MAX_PATCH_LINES: 50",
		"MAX_FILE_PATCH_LINES: 25",
		"CONSTRAINTS:",
		"FAIL_REASON:",
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("executionBlock() missing %q in %q", want, block)
		}
	}
}
