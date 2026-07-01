package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/opd-ai/orchestrator/memory"
)

func writeBuildFailure(taskID, output string) {
	if taskID == "" || output == "" {
		return
	}

	path := filepath.Join("logs", "build_failures", taskID+".log")
	writeArtifact(path, output)
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
`,
		summary.TasksTotal,
		summary.TasksCompleted,
		summary.TasksBlocked,
		summary.DurationSeconds,
		summary.Branch,
	)

	writeArtifact("AUTONOMOUS_RUN_SUMMARY.md", content)
}

func writeArtifact(path, content string) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}

	_ = os.WriteFile(path, []byte(content), 0o644)
}
