package memory

import "fmt"

func SummarizeForPlanner() string {
	m, err := LoadMetricsFromBranch()
	if err != nil || m.TotalRuns == 0 {
		return ""
	}

	lines := baseSummaryLines(m)
	lines = append(lines, optionalFileSummaryLines(m)...)
	lines = append(lines, optionalFailureSummaryLines(m)...)
	if len(lines) > 10 {
		lines = lines[:10]
	}
	return joinSummaryLines(lines)
}

func baseSummaryLines(m AdaptiveMetrics) []string {
	return []string{
		"Recent adaptive metrics:",
		fmt.Sprintf("- Avg successful patch size: %.1f lines", m.AvgSuccessPatchSize),
		fmt.Sprintf("- Avg retries per task: %.2f", m.AvgRetryCount),
		fmt.Sprintf("- Most problematic file: %s", m.MostProblematicFile),
		fmt.Sprintf("- Prefer atomic changes near %.0f lines", m.AvgSuccessPatchSize),
	}
}

func optionalFileSummaryLines(m AdaptiveMetrics) []string {
	if len(m.TopProblemFiles) > 0 {
		return []string{
			fmt.Sprintf("- Top problematic files: %s", formatCountMetrics(m.TopProblemFiles)),
		}
	}
	return nil
}

func optionalFailureSummaryLines(m AdaptiveMetrics) []string {
	if m.MostCommonFailure == "" && len(m.TopFailureTypes) == 0 {
		return nil
	}

	lines := []string{}
	if m.MostCommonFailure != "" {
		lines = append(lines, fmt.Sprintf("- Most common failure: %s", m.MostCommonFailure))
		lines = append(lines, fmt.Sprintf("- Watch for recurring %s failures", m.MostCommonFailure))
	}
	if len(m.TopFailureTypes) > 0 {
		lines = append(lines, fmt.Sprintf("- Top failure types: %s", formatCountMetrics(m.TopFailureTypes)))
	}
	return lines
}

func joinSummaryLines(lines []string) string {
	out := ""
	for i, line := range lines {
		if i > 0 {
			out += "\n"
		}
		out += line
	}
	return out
}

func formatCountMetrics(metrics []CountMetric) string {
	out := ""
	for i, metric := range metrics {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%s (%d)", metric.Name, metric.Count)
	}
	return out
}
