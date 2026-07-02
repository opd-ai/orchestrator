package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

const topTrackedPatterns = 3

// LoadMetrics reads adaptive metrics from the working tree metrics file when it exists.
func LoadMetrics() (AdaptiveMetrics, error) {
	var m AdaptiveMetrics
	data, err := os.ReadFile(MetricsFile)
	if err != nil {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("metrics decode: %w", err)
	}
	return m, nil
}

// LoadMetricsFromBranch reads AdaptiveMetrics directly from the memories
// branch using "git show", without requiring a branch checkout.
// Falls back to LoadMetrics when the branch or file does not yet exist.
func LoadMetricsFromBranch() (AdaptiveMetrics, error) {
	out, err := exec.Command("git", "show", MemoryBranch+":"+MetricsFile).Output()
	if err != nil {
		return LoadMetrics()
	}
	var m AdaptiveMetrics
	if err := json.Unmarshal(out, &m); err != nil {
		return m, err
	}
	return m, nil
}

// SaveMetrics writes adaptive metrics to the working tree metrics file.
func SaveMetrics(updated AdaptiveMetrics) error {
	data, _ := json.MarshalIndent(updated, "", "  ")
	return os.WriteFile(MetricsFile, data, 0644)
}

// UpdateMetrics merges the latest run summary into persisted adaptive metrics on the memory branch.
func UpdateMetrics(summary RunSummary) error {
	return withMemoryWorktree(func(worktreePath string) error {
		metricsPath := filepath.Join(worktreePath, MetricsFile)
		metrics, _ := loadMetricsFromPath(metricsPath)
		updatedMetrics := mergeSummaryMetrics(metrics, summary)

		data, err := json.MarshalIndent(updatedMetrics, "", "  ")
		if err != nil {
			return fmt.Errorf("metrics encode: %w", err)
		}
		if err := os.WriteFile(metricsPath, data, 0o644); err != nil {
			return err
		}

		return commitWorktreeChanges(worktreePath, "memory: update adaptive metrics", true, MetricsFile)
	})
}

func loadMetricsFromPath(path string) (AdaptiveMetrics, error) {
	var m AdaptiveMetrics
	data, err := os.ReadFile(path)
	if err != nil {
		return m, nil
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("metrics decode: %w", err)
	}
	return m, nil
}

// mergeSummaryMetrics folds one run summary into the running adaptive metrics aggregate.
func mergeSummaryMetrics(m AdaptiveMetrics, summary RunSummary) AdaptiveMetrics {
	total := float64(m.TotalRuns)

	m.AvgSuccessPatchSize =
		((m.AvgSuccessPatchSize * total) +
			float64(summary.LargestPatch)) / (total + 1)
	m.AvgRetryCount =
		((m.AvgRetryCount * total) +
			summary.AvgRetries) / (total + 1)
	m.AvgInferenceLatencyMs =
		((m.AvgInferenceLatencyMs * total) +
			summary.AvgInferenceLatencyMs) / (total + 1)
	m.AvgPatchConfidence =
		((m.AvgPatchConfidence * total) +
			summary.AvgPatchConfidence) / (total + 1)
	m.AvgPatchRisk =
		((m.AvgPatchRisk * total) +
			summary.AvgPatchRisk) / (total + 1)
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

// mergeCountMaps adds positive counts from src into dst.
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

// topCountMetrics returns the highest-count metrics entries in descending order.
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
