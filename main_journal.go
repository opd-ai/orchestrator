package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	journalFile          = "orchestrator-journal.json"
	journalStepPatched   = "patched"
	journalStepBuilt     = "built"
	journalStepCommitted = "committed"
)

type executionJournal struct {
	TaskID    string `json:"task_id"`
	Step      string `json:"step"`
	PatchHash string `json:"patch_hash"`
	PatchDiff string `json:"patch_diff,omitempty"`
}

func recordExecutionJournal(taskID, step, diff string) error {
	entry := executionJournal{TaskID: taskID, Step: step}
	prev, ok, err := loadExecutionJournal()
	if err != nil {
		return err
	}
	if ok {
		entry.PatchHash = prev.PatchHash
		entry.PatchDiff = prev.PatchDiff
	}
	if diff != "" {
		entry.PatchHash = hashString(diff)
		entry.PatchDiff = diff
	}
	return writeExecutionJournal(entry)
}

func loadExecutionJournal() (executionJournal, bool, error) {
	var entry executionJournal
	data, err := os.ReadFile(journalFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return entry, false, nil
		}
		return entry, false, err
	}
	if err := json.Unmarshal(data, &entry); err != nil {
		// Treat malformed JSON as an interrupted write: clear the corrupt file
		// and continue rather than propagating the parse error.
		logInfo("journal_corrupt_cleared", "", fmt.Sprintf("invalid JSON treated as interrupted write: %v", err))
		if clearErr := clearExecutionJournal(); clearErr != nil {
			return entry, false, clearErr
		}
		return entry, false, nil
	}
	return entry, true, nil
}

func writeExecutionJournal(entry executionJournal) error {
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(journalFile)
	tmpFile, err := os.CreateTemp(dir, ".orchestrator-journal-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, journalFile); err != nil {
		return err
	}
	cleanup = false
	return nil
}

func clearExecutionJournal() error {
	if err := os.Remove(journalFile); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func recoverExecutionJournal() error {
	entry, ok, err := loadExecutionJournal()
	if err != nil || !ok {
		return err
	}
	if entry.TaskID == "" || entry.Step == "" {
		// Incomplete payload — treat as an interrupted write and clear.
		logInfo("journal_incomplete_cleared", "", "incomplete journal payload treated as interrupted write")
		return clearExecutionJournal()
	}

	switch entry.Step {
	case journalStepPatched:
		if entry.PatchDiff != "" {
			if err := revertPatch(entry.PatchDiff); err != nil {
				return err
			}
		}
		if err := updateTaskStatus(entry.TaskID, "pending"); err != nil {
			return err
		}
		logInfo("journal_recovered_reverted", entry.TaskID, "")
		return clearExecutionJournal()
	case journalStepBuilt:
		return recoverBuiltJournal(entry)
	case journalStepCommitted:
		return clearExecutionJournal()
	default:
		return fmt.Errorf("unknown journal step %q", entry.Step)
	}
}

func recoverBuiltJournal(entry executionJournal) error {
	task, err := taskForRecovery(entry.TaskID)
	if err != nil {
		return err
	}
	files, err := validateRecoveredBuiltPatch(entry)
	if err != nil {
		return err
	}
	if err := gitCommitFiles(task, files); err != nil {
		return err
	}
	if err := updateTaskStatus(entry.TaskID, "complete"); err != nil {
		return err
	}
	if err := recordExecutionJournal(task.ID, journalStepCommitted, ""); err != nil {
		return err
	}
	logInfo("journal_recovered_committed", entry.TaskID, "")
	return clearExecutionJournal()
}

func validateRecoveredBuiltPatch(entry executionJournal) ([]string, error) {
	if entry.PatchDiff == "" {
		return nil, errors.New("journal missing patch diff")
	}
	if got := hashString(entry.PatchDiff); got != entry.PatchHash {
		return nil, fmt.Errorf("journal patch hash mismatch: got %s want %s", got, entry.PatchHash)
	}
	files := filesTouched(entry.PatchDiff)
	if len(files) == 0 {
		return nil, errors.New("journal patch touches no files")
	}
	if err := reversePatchDryRun(entry.PatchDiff); err != nil {
		return nil, err
	}
	return files, nil
}

func reversePatchDryRun(diff string) error {
	tmpFile, err := os.CreateTemp("", "orchestrator-recovery-*.patch")
	if err != nil {
		return fmt.Errorf("create temporary recovery patch: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()
	if _, err := tmpFile.WriteString(diff); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	out, err := exec.Command("patch", "-p1", "-R", "--dry-run", "-i", tmpName).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("workspace validation failed: recovered patch no longer matches workspace: %s", msg)
	}
	return nil
}

func taskForRecovery(taskID string) (*Task, error) {
	task := &Task{ID: taskID, Description: fmt.Sprintf("recover interrupted task %s", taskID)}
	tf, ok, err := loadTasksForRecovery()
	if err != nil || !ok {
		return task, err
	}
	for i := range tf.Tasks {
		if tf.Tasks[i].ID == taskID {
			return &tf.Tasks[i], nil
		}
	}
	return task, nil
}

func updateTaskStatus(taskID, status string) error {
	tf, ok, err := loadTasksForRecovery()
	if err != nil || !ok {
		return err
	}
	for i := range tf.Tasks {
		if tf.Tasks[i].ID == taskID {
			tf.Tasks[i].Status = status
			mustSaveTasks(tf)
			return nil
		}
	}
	return nil
}

func loadTasksForRecovery() (TaskFile, bool, error) {
	var tf TaskFile
	data, err := os.ReadFile(tasksFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return tf, false, nil
		}
		return tf, false, err
	}
	if err := json.Unmarshal(data, &tf); err != nil {
		return tf, false, err
	}
	return tf, true, nil
}
