package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SaveRun persists one run summary on the memory branch and trims old summaries.
func SaveRun(summary RunSummary) error {
	return withMemoryWorktree(func(worktreePath string) error {
		runsPath := filepath.Join(worktreePath, RunsDir)
		if err := os.MkdirAll(runsPath, 0o755); err != nil {
			return fmt.Errorf("runs dir: %w", err)
		}

		filename := filepath.Join(runsPath,
			fmt.Sprintf("%s.json",
				summary.Timestamp.Format("2006-01-02T15-04-05")))

		data, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("run summary encode: %w", err)
		}
		if err := os.WriteFile(filename, data, 0o644); err != nil {
			return fmt.Errorf("write run summary: %w", err)
		}

		trimOldRuns(runsPath)
		return commitWorktreeChanges(worktreePath, "memory: add run summary", false, RunsDir)
	})
}

// trimOldRuns removes the oldest stored run summaries beyond the retention limit.
func trimOldRuns(runsPath string) {
	files, err := os.ReadDir(runsPath)
	if err != nil || len(files) <= MaxStoredRuns {
		return
	}

	excess := len(files) - MaxStoredRuns
	for i := 0; i < excess; i++ {
		os.Remove(filepath.Join(runsPath, files[i].Name()))
	}
}
