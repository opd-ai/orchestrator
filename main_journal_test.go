package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRecordExecutionJournalRoundTrip(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	diff := strings.Join([]string{
		"diff --git a/sample.txt b/sample.txt",
		"--- a/sample.txt",
		"+++ b/sample.txt",
		"@@ -1 +1 @@",
		"-a",
		"+b",
	}, "\n") + "\n"

	if err := recordExecutionJournal("T1", journalStepPatched, diff); err != nil {
		t.Fatalf("recordExecutionJournal: %v", err)
	}

	entry, ok, err := loadExecutionJournal()
	if err != nil {
		t.Fatalf("loadExecutionJournal: %v", err)
	}
	if !ok {
		t.Fatal("expected journal entry")
	}
	if entry.TaskID != "T1" || entry.Step != journalStepPatched {
		t.Fatalf("unexpected entry: %#v", entry)
	}
	if entry.PatchHash == "" || entry.PatchDiff != diff {
		t.Fatalf("expected patch hash/diff persisted, got %#v", entry)
	}
}

func TestRecoverExecutionJournalPatchedRevertsWorkspace(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	path := filepath.Join(tmpDir, "sample.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	tf := TaskFile{Tasks: []Task{{ID: "T1", Description: "change sample", Status: "in_progress"}}}
	saveTasks(tf)

	diff := strings.Join([]string{
		"diff --git a/sample.txt b/sample.txt",
		"--- a/sample.txt",
		"+++ b/sample.txt",
		"@@ -1 +1 @@",
		"-a",
		"+b",
	}, "\n") + "\n"

	if err := applyPatch(diff); err != nil {
		t.Fatalf("applyPatch: %v", err)
	}
	if err := recordExecutionJournal("T1", journalStepPatched, diff); err != nil {
		t.Fatalf("recordExecutionJournal: %v", err)
	}

	if err := recoverExecutionJournal(); err != nil {
		t.Fatalf("recoverExecutionJournal: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(got) != "a\n" {
		t.Fatalf("expected reverted file, got %q", got)
	}
	if _, err := os.Stat(journalFile); !os.IsNotExist(err) {
		t.Fatalf("expected journal cleared, stat err=%v", err)
	}
	tf = loadTasks()
	if tf.Tasks[0].Status != "pending" {
		t.Fatalf("expected task set to pending, got %q", tf.Tasks[0].Status)
	}
}

func TestRecoverExecutionJournalBuiltCommitsPatch(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	initGitRepo(t, tmpDir)

	path := filepath.Join(tmpDir, "sample.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("orchestrator-journal.json\n"), 0o644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}
	tf := TaskFile{Tasks: []Task{{ID: "T1", Description: "change sample", Status: "in_progress"}}}
	saveTasks(tf)
	runCmd(t, "git", "add", ".")
	runCmd(t, "git", "commit", "-m", "initial")

	diff := strings.Join([]string{
		"diff --git a/sample.txt b/sample.txt",
		"--- a/sample.txt",
		"+++ b/sample.txt",
		"@@ -1 +1 @@",
		"-a",
		"+b",
	}, "\n") + "\n"
	if err := applyPatch(diff); err != nil {
		t.Fatalf("applyPatch: %v", err)
	}
	if err := recordExecutionJournal("T1", journalStepBuilt, diff); err != nil {
		t.Fatalf("recordExecutionJournal: %v", err)
	}

	if err := recoverExecutionJournal(); err != nil {
		t.Fatalf("recoverExecutionJournal: %v", err)
	}

	tf = loadTasks()
	if tf.Tasks[0].Status != "complete" {
		t.Fatalf("expected recovered task complete, got %q", tf.Tasks[0].Status)
	}
	if _, err := os.Stat(journalFile); !os.IsNotExist(err) {
		t.Fatalf("expected journal cleared, stat err=%v", err)
	}

	logOut := runCmd(t, "git", "log", "--oneline", "-1")
	if !strings.Contains(logOut, "Task T1: change sample") {
		t.Fatalf("expected recovery commit message, got %q", logOut)
	}
	// tasks.json is updated after gitCommit to avoid marking complete on a failed
	// commit, so it may remain modified in the working tree.
	statusOut := runCmd(t, "git", "status", "--short")
	if strings.Contains(statusOut, "sample.txt") {
		t.Fatalf("expected sample.txt committed, got git status %q", statusOut)
	}
}
