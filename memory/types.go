package memory

import "time"

const (
	MemoryBranch  = "memories"
	RunsDir       = "runs"
	MetricsFile   = "adaptive_metrics.json"
	MaxStoredRuns = 20
)

type RunSummary struct {
	Timestamp               time.Time      `json:"timestamp"`
	Branch                  string         `json:"branch"`
	DurationSeconds         int64          `json:"duration_seconds"`
	TasksTotal              int            `json:"tasks_total"`
	TasksCompleted          int            `json:"tasks_completed"`
	TasksBlocked            int            `json:"tasks_blocked"`
	AvgRetries              float64        `json:"avg_retries"`
	LargestPatch            int            `json:"largest_patch"`
	MostModifiedFile        string         `json:"most_modified_file"`
	MostCommonFailure       string         `json:"most_common_failure"`
	RetryConvergenceSamples int            `json:"retry_convergence_samples"`
	RetryConvergenceAlerts  int            `json:"retry_convergence_alerts"`
	FailurePatterns         map[string]int `json:"failure_patterns,omitempty"`
	ModifiedFiles           map[string]int `json:"modified_files,omitempty"`
}

type CountMetric struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type AdaptiveMetrics struct {
	AvgSuccessPatchSize float64        `json:"avg_success_patch_size"`
	AvgRetryCount       float64        `json:"avg_retry_count"`
	MostProblematicFile string         `json:"most_problematic_file"`
	MostCommonFailure   string         `json:"most_common_failure"`
	TotalRuns           int            `json:"total_runs"`
	FailureCounts       map[string]int `json:"failure_counts,omitempty"`
	ProblemFileCounts   map[string]int `json:"problem_file_counts,omitempty"`
	TopFailureTypes     []CountMetric  `json:"top_failure_types,omitempty"`
	TopProblemFiles     []CountMetric  `json:"top_problem_files,omitempty"`
}
