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
	if entry.PatchSHA256 == "" || len(entry.PatchSHA256) != 64 {
		t.Fatalf("expected sha256 patch digest persisted, got %#v", entry)
	}
	if len(entry.TouchedFiles) != 1 || entry.TouchedFiles[0] != "sample.txt" {
		t.Fatalf("expected touched files persisted, got %#v", entry.TouchedFiles)
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
	if err := saveTasks(tf); err != nil {
		t.Fatalf("saveTasks() error = %v", err)
	}

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
	if err := saveTasks(tf); err != nil {
		t.Fatalf("saveTasks() error = %v", err)
	}
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

func TestRecoverExecutionJournalBuiltSkipsUnrelatedWorkspaceChanges(t *testing.T) {
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
	otherPath := filepath.Join(tmpDir, "other.txt")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatalf("write sample: %v", err)
	}
	if err := os.WriteFile(otherPath, []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write other: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("orchestrator-journal.json\n"), 0o644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}
	if err := saveTasks(TaskFile{Tasks: []Task{{ID: "T1", Description: "change sample", Status: "in_progress"}}}); err != nil {
		t.Fatalf("saveTasks() error = %v", err)
	}
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
	if err := os.WriteFile(otherPath, []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write other dirty: %v", err)
	}
	if err := recordExecutionJournal("T1", journalStepBuilt, diff); err != nil {
		t.Fatalf("recordExecutionJournal: %v", err)
	}

	if err := recoverExecutionJournal(); err != nil {
		t.Fatalf("recoverExecutionJournal: %v", err)
	}

	statusOut := runCmd(t, "git", "status", "--short")
	if strings.Contains(statusOut, "sample.txt") {
		t.Fatalf("expected sample.txt committed, got git status %q", statusOut)
	}
	if !strings.Contains(statusOut, "other.txt") {
		t.Fatalf("expected unrelated file to remain dirty, got git status %q", statusOut)
	}

	otherHead := runCmd(t, "git", "show", "HEAD:other.txt")
	if otherHead != "base\n" {
		t.Fatalf("expected other.txt unchanged in HEAD, got %q", otherHead)
	}
}

func TestRecoverExecutionJournalBuiltFailsOnWorkspaceMismatch(t *testing.T) {
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
		t.Fatalf("write sample: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, ".gitignore"), []byte("orchestrator-journal.json\n"), 0o644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}
	if err := saveTasks(TaskFile{Tasks: []Task{{ID: "T1", Description: "change sample", Status: "in_progress"}}}); err != nil {
		t.Fatalf("saveTasks() error = %v", err)
	}
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
	if err := os.WriteFile(path, []byte("c\n"), 0o644); err != nil {
		t.Fatalf("mutate sample: %v", err)
	}

	err = recoverExecutionJournal()
	if err == nil {
		t.Fatal("expected workspace mismatch to fail recovery")
	}
	if !strings.Contains(err.Error(), "recovered patch no longer matches workspace") {
		t.Fatalf("expected workspace mismatch error, got %v", err)
	}

	logOut := runCmd(t, "git", "log", "--oneline", "-1")
	if strings.Contains(logOut, "Task T1: change sample") {
		t.Fatalf("expected no recovery commit, got %q", logOut)
	}
}

func TestRecoverExecutionJournalCorruptJSONClearsAndContinues(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	// Write a corrupt (non-JSON) journal file simulating an interrupted write.
	if err := os.WriteFile(journalFile, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write corrupt journal: %v", err)
	}

	if err := recoverExecutionJournal(); err != nil {
		t.Fatalf("expected corrupt journal to be cleared without error, got: %v", err)
	}
	if _, statErr := os.Stat(journalFile); !os.IsNotExist(statErr) {
		t.Fatalf("expected corrupt journal file to be cleared, stat err=%v", statErr)
	}
}

func TestRecoverExecutionJournalIncompletePayloadClearsAndContinues(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	// Write a syntactically valid JSON payload with missing required fields.
	if err := os.WriteFile(journalFile, []byte(`{"task_id":"","step":""}`), 0o644); err != nil {
		t.Fatalf("write incomplete journal: %v", err)
	}

	if err := recoverExecutionJournal(); err != nil {
		t.Fatalf("expected incomplete journal to be cleared without error, got: %v", err)
	}
	if _, statErr := os.Stat(journalFile); !os.IsNotExist(statErr) {
		t.Fatalf("expected incomplete journal file to be cleared, stat err=%v", statErr)
	}
}

func TestRecoverExecutionJournalBuiltSupportsLegacyPatchHash(t *testing.T) {
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
	if err := saveTasks(TaskFile{Tasks: []Task{{ID: "T1", Description: "change sample", Status: "in_progress"}}}); err != nil {
		t.Fatalf("saveTasks() error = %v", err)
	}
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
	entry := executionJournal{
		TaskID:    "T1",
		Step:      journalStepBuilt,
		PatchHash: hashString(diff),
		PatchDiff: diff,
	}
	if err := writeExecutionJournal(entry); err != nil {
		t.Fatalf("writeExecutionJournal: %v", err)
	}

	if err := recoverExecutionJournal(); err != nil {
		t.Fatalf("recoverExecutionJournal: %v", err)
	}

	logOut := runCmd(t, "git", "log", "--oneline", "-1")
	if !strings.Contains(logOut, "Task T1: change sample") {
		t.Fatalf("expected recovery commit message, got %q", logOut)
	}
}

func TestHashSHA256NormalizedPreservesCaseSensitivity(t *testing.T) {
	upper := "diff --git a/x b/x\n+ABC\n"
	lower := "diff --git a/x b/x\n+abc\n"
	if hashSHA256Normalized(upper) == hashSHA256Normalized(lower) {
		t.Fatal("expected sha256 journal digest to remain case-sensitive")
	}
}
