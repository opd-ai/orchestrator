package memory

import "testing"

func TestMergeSummaryMetricsTracksTopPatterns(t *testing.T) {
	current := AdaptiveMetrics{
		TotalRuns:         1,
		FailureCounts:     map[string]int{"unused import": 2},
		ProblemFileCounts: map[string]int{"main.go": 3},
	}
	summary := RunSummary{
		LargestPatch:    40,
		AvgRetries:      1.5,
		FailurePatterns: map[string]int{"unused import": 1, "undefined symbol": 4},
		ModifiedFiles:   map[string]int{"main.go": 1, "main_exec.go": 5},
	}

	merged := mergeSummaryMetrics(current, summary)
	if got := merged.FailureCounts["unused import"]; got != 3 {
		t.Fatalf("FailureCounts[unused import] = %d, want 3", got)
	}
	if got := merged.ProblemFileCounts["main_exec.go"]; got != 5 {
		t.Fatalf("ProblemFileCounts[main_exec.go] = %d, want 5", got)
	}
	if len(merged.TopFailureTypes) == 0 || merged.TopFailureTypes[0].Name != "undefined symbol" {
		t.Fatalf("TopFailureTypes = %#v, want undefined symbol first", merged.TopFailureTypes)
	}
	if len(merged.TopProblemFiles) == 0 || merged.TopProblemFiles[0].Name != "main_exec.go" {
		t.Fatalf("TopProblemFiles = %#v, want main_exec.go first", merged.TopProblemFiles)
	}
}
