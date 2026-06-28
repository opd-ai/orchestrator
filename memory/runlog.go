package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func SaveRun(summary RunSummary) error {
	originalBranch, err := currentBranch()
	if err != nil {
		return err
	}

	if err := ensureMemoryBranch(); err != nil {
		return err
	}

	os.MkdirAll(RunsDir, 0755)

	filename := filepath.Join(RunsDir,
		fmt.Sprintf("%s.json",
			summary.Timestamp.Format("2006-01-02T15-04-05")))

	data, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}

	trimOldRuns()

	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "memory: add run summary").Run()

	return checkoutBranch(originalBranch)
}

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
