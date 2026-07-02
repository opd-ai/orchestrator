package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// SaveRun persists one run summary on the memory branch and trims old summaries.
func SaveRun(summary RunSummary) error {
	originalBranch, err := currentBranch()
	if err != nil {
		return err
	}

	if err := ensureMemoryBranch(); err != nil {
		return err
	}

	if err := os.MkdirAll(RunsDir, 0755); err != nil {
		return fmt.Errorf("runs dir: %w", err)
	}

	filename := filepath.Join(RunsDir,
		fmt.Sprintf("%s.json",
			summary.Timestamp.Format("2006-01-02T15-04-05")))

	data, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}

	trimOldRuns()

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if err := exec.Command("git", "commit", "-m", "memory: add run summary").Run(); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}

	return checkoutBranch(originalBranch)
}

// trimOldRuns removes the oldest stored run summaries beyond the retention limit.
func trimOldRuns() {
	files, err := os.ReadDir(RunsDir)
	if err != nil || len(files) <= MaxStoredRuns {
		return
	}

	excess := len(files) - MaxStoredRuns
	for i := 0; i < excess; i++ {
		os.Remove(filepath.Join(RunsDir, files[i].Name()))
	}
}
