package main

import (
	"fmt"
	"time"

	"github.com/opd-ai/orchestrator/memory"
)

func runExecutionMode() {
	start := time.Now()

	// Inject memory into planner
	memoryContext := memory.SummarizeForPlanner()
	injectMemoryIntoPlanner(memoryContext)

	// Placeholder: execution loop would run here
	fmt.Println("Execution mode started.")
	fmt.Println("Model:", modelName)

	// Save memory summary at end
	summary := memory.RunSummary{
		Timestamp:       time.Now(),
		Branch:          currentGitBranch(),
		DurationSeconds: int64(time.Since(start).Seconds()),
	}

	memory.SaveRun(summary)
	memory.UpdateMetrics(summary)
}
