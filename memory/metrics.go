package memory

import (
	"encoding/json"
	"os"
	"os/exec"
)

func LoadMetrics() (AdaptiveMetrics, error) {
	var m AdaptiveMetrics
	data, err := os.ReadFile(MetricsFile)
	if err != nil {
		return m, nil
	}
	json.Unmarshal(data, &m)
	return m, nil
}

func SaveMetrics(updated AdaptiveMetrics) error {
	data, _ := json.MarshalIndent(updated, "", "  ")
	return os.WriteFile(MetricsFile, data, 0644)
}

func UpdateMetrics(summary RunSummary) error {
	originalBranch, err := currentBranch()
	if err != nil {
		return err
	}

	if err := ensureMemoryBranch(); err != nil {
		return err
	}

	m, _ := LoadMetrics()

	total := float64(m.TotalRuns)

	m.AvgSuccessPatchSize =
		((m.AvgSuccessPatchSize * total) +
			float64(summary.LargestPatch)) / (total + 1)

	m.AvgRetryCount =
		((m.AvgRetryCount * total) +
			summary.AvgRetries) / (total + 1)

	m.MostProblematicFile = summary.MostModifiedFile
	if summary.MostCommonFailure != "" {
		m.MostCommonFailure = summary.MostCommonFailure
	}
	m.TotalRuns++

	if err := SaveMetrics(m); err != nil {
		return err
	}

	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "memory: update adaptive metrics").Run()

	return checkoutBranch(originalBranch)
}
