package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opd-ai/orchestrator/memory"
)

// buildFailureArtifact is the structured JSON envelope written to
// logs/build_failures/<task_id>.json for each build failure.
type buildFailureArtifact struct {
	TaskID        string   `json:"task_id"`
	Timestamp     string   `json:"timestamp"`
	Retry         int      `json:"retry"`
	ErrorCategory string   `json:"error_category"`
	ErrorLines    []string `json:"error_lines"`
	Raw           string   `json:"raw"`
}

// writeBuildFailure writes a structured JSON artifact for a build failure.
// retry is the task's RetryCount at the time of failure (0 for the first attempt).
func writeBuildFailure(taskID, output string, retry int) {
	if taskID == "" || output == "" {
		return
	}

	artifact := buildFailureArtifact{
		TaskID:        taskID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Retry:         retry,
		ErrorCategory: classifyBuildFailure(output),
		ErrorLines:    compilerErrorLines(output),
		Raw:           output,
	}
	data, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		logError("build_failure_marshal_failed", taskID, err.Error())
		return
	}

	path := filepath.Join("logs", "build_failures", taskID+".json")
	writeArtifact(path, string(data))
}

func writeRejectedPatch(taskID, diff string) {
	if taskID == "" || diff == "" {
		return
	}

	path := filepath.Join("logs", "rejected_patches", taskID+".diff")
	writeArtifact(path, diff)
}

func writeRunSummary(summary memory.RunSummary) {
	content := fmt.Sprintf(`# AUTONOMOUS RUN SUMMARY

- Total tasks: %d
- Completed tasks: %d
- Blocked tasks: %d
- Execution duration: %ds
- Git branch: %s
- Retry convergence alerts: %d/%d
`,
		summary.TasksTotal,
		summary.TasksCompleted,
		summary.TasksBlocked,
		summary.DurationSeconds,
		summary.Branch,
		summary.RetryConvergenceAlerts,
		summary.RetryConvergenceSamples,
	)

	writeArtifact("AUTONOMOUS_RUN_SUMMARY.md", content)
}

func writeArtifact(path, content string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		logError("artifact_mkdir_failed", path, err.Error())
		return
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		logError("artifact_write_failed", path, err.Error())
	}
}
