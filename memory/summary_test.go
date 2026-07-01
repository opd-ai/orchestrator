package memory

import (
	"os"
	"strings"
	"testing"
)

func TestSummarizeForPlannerIsCompressed(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Remove(MetricsFile)
	})

	err := SaveMetrics(AdaptiveMetrics{
		AvgSuccessPatchSize: 42.5,
		AvgRetryCount:       1.75,
		MostProblematicFile: "main_exec.go",
		MostCommonFailure:   "unused import",
		TotalRuns:           3,
	})
	if err != nil {
		t.Fatalf("SaveMetrics() error = %v", err)
	}

	summary := SummarizeForPlanner()
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if got := len(strings.Split(summary, "\n")); got > 10 {
		t.Fatalf("expected <=10 lines, got %d: %q", got, summary)
	}
}
