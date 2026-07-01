package memory

import "fmt"

func SummarizeForPlanner() string {
	m, err := LoadMetrics()
	if err != nil || m.TotalRuns == 0 {
		return ""
	}

	lines := []string{
		"Recent adaptive metrics:",
		fmt.Sprintf("- Avg successful patch size: %.1f lines", m.AvgSuccessPatchSize),
		fmt.Sprintf("- Avg retries per task: %.2f", m.AvgRetryCount),
		fmt.Sprintf("- Most problematic file: %s", m.MostProblematicFile),
		fmt.Sprintf("- Prefer atomic changes near %.0f lines", m.AvgSuccessPatchSize),
	}
	if m.MostCommonFailure != "" {
		lines = append(lines, fmt.Sprintf("- Most common failure: %s", m.MostCommonFailure))
		lines = append(lines, fmt.Sprintf("- Watch for recurring %s failures", m.MostCommonFailure))
	}
	if len(lines) > 10 {
		lines = lines[:10]
	}
	return joinSummaryLines(lines)
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
