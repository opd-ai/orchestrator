package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
