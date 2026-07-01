package memory

import "fmt"

func SummarizeForPlanner() string {
	m, err := LoadMetrics()
	if err != nil || m.TotalRuns == 0 {
		return ""
	}

	s := fmt.Sprintf(`
Recent adaptive metrics from prior runs:
- Average successful patch size: %.1f lines
- Average retries per task: %.2f
- Most problematic file: %s
`,
		m.AvgSuccessPatchSize,
		m.AvgRetryCount,
		m.MostProblematicFile,
	)

	if m.MostCommonFailure != "" {
		s += fmt.Sprintf("- Most common failure: %s\n", m.MostCommonFailure)
	}

	s += fmt.Sprintf(`
Guidance:
- Prefer atomic changes near %.0f lines.
- Avoid repeated modification of %s unless necessary.
`,
		m.AvgSuccessPatchSize,
		m.MostProblematicFile,
	)

	if m.MostCommonFailure != "" {
		s += fmt.Sprintf("- Watch for recurring %s failures.\n", m.MostCommonFailure)
	}

	return s
}
