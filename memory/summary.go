package memory

import "fmt"

func SummarizeForPlanner() string {
	m, err := LoadMetrics()
	if err != nil || m.TotalRuns == 0 {
		return ""
	}

	return fmt.Sprintf(`
Recent adaptive metrics from prior runs:
- Average successful patch size: %.1f lines
- Average retries per task: %.2f
- Most problematic file: %s

Guidance:
- Prefer atomic changes near %.0f lines.
- Avoid repeated modification of %s unless necessary.
`,
		m.AvgSuccessPatchSize,
		m.AvgRetryCount,
		m.MostProblematicFile,
		m.AvgSuccessPatchSize,
		m.MostProblematicFile,
	)
}
