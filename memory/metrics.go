package memory

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"sort"
)

const topTrackedPatterns = 3

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

	metrics, _ := LoadMetrics()
	updatedMetrics := mergeSummaryMetrics(metrics, summary)

	if err := SaveMetrics(updatedMetrics); err != nil {
		return err
	}

	if err := exec.Command("git", "add", ".").Run(); err != nil {
		return err
	}
	if err := exec.Command("git", "commit", "-m", "memory: update adaptive metrics").Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return err
		}
	}

	return checkoutBranch(originalBranch)
}

func mergeSummaryMetrics(m AdaptiveMetrics, summary RunSummary) AdaptiveMetrics {
	total := float64(m.TotalRuns)

	m.AvgSuccessPatchSize =
		((m.AvgSuccessPatchSize * total) +
			float64(summary.LargestPatch)) / (total + 1)
	m.AvgRetryCount =
		((m.AvgRetryCount * total) +
			summary.AvgRetries) / (total + 1)
	if summary.MostModifiedFile != "" {
		m.MostProblematicFile = summary.MostModifiedFile
	}
	if summary.MostCommonFailure != "" {
		m.MostCommonFailure = summary.MostCommonFailure
	}
	m.FailureCounts = mergeCountMaps(m.FailureCounts, summary.FailurePatterns)
	m.ProblemFileCounts = mergeCountMaps(m.ProblemFileCounts, summary.ModifiedFiles)
	m.TopFailureTypes = topCountMetrics(m.FailureCounts, topTrackedPatterns)
	m.TopProblemFiles = topCountMetrics(m.ProblemFileCounts, topTrackedPatterns)
	m.TotalRuns++

	return m
}

func mergeCountMaps(dst, src map[string]int) map[string]int {
	if len(src) == 0 {
		return dst
	}
	if dst == nil {
		dst = make(map[string]int, len(src))
	}
	for name, count := range src {
		if count <= 0 {
			continue
		}
		dst[name] += count
	}
	return dst
}

func topCountMetrics(counts map[string]int, limit int) []CountMetric {
	if len(counts) == 0 || limit <= 0 {
		return nil
	}

	metrics := make([]CountMetric, 0, len(counts))
	for name, count := range counts {
		if count > 0 {
			metrics = append(metrics, CountMetric{Name: name, Count: count})
		}
	}

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].Count == metrics[j].Count {
			return metrics[i].Name < metrics[j].Name
		}
		return metrics[i].Count > metrics[j].Count
	})

	if len(metrics) > limit {
		metrics = metrics[:limit]
	}
	return metrics
}
