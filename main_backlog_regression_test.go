package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSubsystemMapIncludesPendingOnly(t *testing.T) {
	tasks := []Task{
		{ID: "A", Status: "pending", Files: []string{"main.go"}},
		{ID: "B", Status: "", Files: []string{"main.go"}},
		{ID: "C", Status: "complete", Files: []string{"main_exec.go"}},
		{ID: "D", Status: "pending", Files: []string{"memory/metrics.go"}},
	}

	got := buildSubsystemMap(tasks)
	if len(got["root"]) != 1 || got["root"][0] != 0 {
		t.Fatalf("root subsystem indices = %#v, want [0]", got["root"])
	}
	if len(got["memory"]) != 1 || got["memory"][0] != 3 {
		t.Fatalf("memory subsystem indices = %#v, want [3]", got["memory"])
	}
}

func TestEnsureBranchReturnsErrorOnCheckoutFailure(t *testing.T) {
	original := gitCheckoutNewBranch
	t.Cleanup(func() {
		gitCheckoutNewBranch = original
	})

	gitCheckoutNewBranch = func(string) error {
		return errors.New("checkout failed")
	}

	err := ensureBranch()
	if err == nil {
		t.Fatal("ensureBranch() error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "checkout failed") {
		t.Fatalf("ensureBranch() error = %q, want wrapped checkout error", err.Error())
	}
}

func TestSaveTasksReturnsErrorOnWriteFailure(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	tmp := t.TempDir()
	locked := filepath.Join(tmp, "readonly")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatalf("Mkdir(readonly) error = %v", err)
	}
	if err := os.Chdir(locked); err != nil {
		t.Fatalf("Chdir(readonly) error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
		_ = os.Chmod(locked, 0o755)
	})

	if err := saveTasks(TaskFile{Tasks: []Task{{ID: "T1", Status: "pending"}}}); err == nil {
		t.Fatal("saveTasks() error = nil, want non-nil")
	}
}
