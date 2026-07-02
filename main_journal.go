package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
		return entry, false, err
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
		task, err := taskForRecovery(entry.TaskID)
		if err != nil {
			return err
		}
		if err := gitCommit(task); err != nil {
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
	case journalStepCommitted:
		return clearExecutionJournal()
	default:
		return fmt.Errorf("unknown journal step %q", entry.Step)
	}
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
			saveTasks(tf)
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
