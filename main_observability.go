package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/opd-ai/orchestrator/memory"
)

// maxArtifactsPerDir is the maximum number of files retained in each
// observability artifact directory. When a new artifact is written, any files
// beyond this limit are removed in lexical (oldest-first) order.
const maxArtifactsPerDir = 50
const artifactTimestampFormat = "20060102T150405.000000000Z"

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

	path := filepath.Join("logs", "build_failures", buildFailureArtifactName(taskID, retry))
	writeArtifact(path, string(data))
}

func writeRejectedPatch(taskID, diff string) {
	if taskID == "" || diff == "" {
		return
	}

	path := filepath.Join("logs", "rejected_patches", rejectedPatchArtifactName(taskID))
	writeArtifact(path, diff)
}

func recordRejectedPatch(taskID, diff, reason, event string, asError bool) {
	writeRejectedPatch(taskID, diff)
	if reason == "" {
		return
	}
	if asError {
		logError(event, taskID, reason)
		return
	}
	logInfo(event, taskID, reason)
}

var artifactNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func buildFailureArtifactName(taskID string, retry int) string {
	return fmt.Sprintf("%s-attempt-%d-%s.json", sanitizeArtifactID(taskID), retry+1, artifactTimestamp())
}

func rejectedPatchArtifactName(taskID string) string {
	return fmt.Sprintf("%s-%s.diff", sanitizeArtifactID(taskID), artifactTimestamp())
}

func sanitizeArtifactID(taskID string) string {
	safe := artifactNameSanitizer.ReplaceAllString(taskID, "_")
	if safe == "" {
		return "task"
	}
	return safe
}

func artifactTimestamp() string {
	return time.Now().UTC().Format(artifactTimestampFormat)
}

func writeRunSummary(summary memory.RunSummary) {
	content := fmt.Sprintf(`# AUTONOMOUS RUN SUMMARY

- Total tasks: %d
- Completed tasks: %d
- Blocked tasks: %d
- Execution duration: %ds
- Git branch: %s
- Retry convergence alerts: %d/%d
%s
`,
		summary.TasksTotal,
		summary.TasksCompleted,
		summary.TasksBlocked,
		summary.DurationSeconds,
		summary.Branch,
		summary.RetryConvergenceAlerts,
		summary.RetryConvergenceSamples,
		blockedReasonsSection(summary.BlockedTaskReasons),
	)

	writeArtifact("AUTONOMOUS_RUN_SUMMARY.md", content)
}

func blockedReasonsSection(reasons map[string]int) string {
	if len(reasons) == 0 {
		return ""
	}
	lines := make([]string, 0, len(reasons))
	for reason, count := range reasons {
		lines = append(lines, fmt.Sprintf("- %s: %d", reason, count))
	}
	sort.Strings(lines)
	return "\n## Blocked task reasons\n\n" + strings.Join(lines, "\n")
}

func writeArtifact(path, content string) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		logError("artifact_mkdir_failed", path, err.Error())
		return
	}

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		logError("artifact_write_failed", path, err.Error())
		return
	}
	pruneArtifactDir(dir, maxArtifactsPerDir)
}

// pruneArtifactDir removes the oldest files in dir until at most maxFiles remain.
// Files are sorted lexically by name; the first N files in that ordering (which
// corresponds to chronological order for timestamp-prefixed or task-ID–prefixed
// names) are removed to bring the directory within the retention limit. Errors
// are logged but do not halt execution.
func pruneArtifactDir(dir string, maxFiles int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	// Collect regular files only.
	files := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	if len(files) <= maxFiles {
		return
	}
	sort.Strings(files)
	for _, f := range files[:len(files)-maxFiles] {
		if err := os.Remove(f); err != nil {
			logError("artifact_prune_failed", f, err.Error())
		}
	}
}
