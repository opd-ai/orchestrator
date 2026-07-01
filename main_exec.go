package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/opd-ai/orchestrator/memory"
)

type executionStats struct {
	tasksTotal      int
	tasksCompleted  int
	tasksBlocked    int
	totalRetries    int
	largestPatch    int
	modifiedFiles   map[string]int
	failurePatterns map[string]int
}

func runExecutionMode() {
	start := time.Now()

	// Inject memory into planner
	memoryContext := memory.SummarizeForPlanner()
	injectMemoryIntoPlanner(memoryContext)

	fmt.Println("Execution mode started.")
	fmt.Println("Model:", modelName)
	stats := execute()

	// Save memory summary at end
	summary := memory.RunSummary{
		Timestamp:         time.Now(),
		Branch:            currentGitBranch(),
		DurationSeconds:   int64(time.Since(start).Seconds()),
		TasksTotal:        stats.tasksTotal,
		TasksCompleted:    stats.tasksCompleted,
		TasksBlocked:      stats.tasksBlocked,
		AvgRetries:        averageRetries(stats.totalRetries, stats.tasksTotal),
		LargestPatch:      stats.largestPatch,
		MostModifiedFile:  mostModifiedFile(stats.modifiedFiles),
		MostCommonFailure: mostCommonFailure(stats.failurePatterns),
	}

	memory.SaveRun(summary)
	memory.UpdateMetrics(summary)
}

func execute() executionStats {
	start := time.Now()
	taskCounter := 0
	stats := executionStats{
		modifiedFiles:   make(map[string]int),
		failurePatterns: make(map[string]int),
	}

	if !resumeBranch {
		ensureBranch()
	}

	ensureTasksFile()

	for {
		if maxRuntime > 0 && time.Since(start) > maxRuntime {
			logInfo("max_runtime_reached", "", "")
			return stats
		}

		if maxTasks > 0 && taskCounter >= maxTasks {
			logInfo("max_tasks_reached", "", "")
			return stats
		}

		tf := loadTasks()
		task := nextExecutableTask(&tf)
		if task == nil {
			logInfo("run_complete", "", "All tasks complete")
			return stats
		}

		taskCounter++
		stats.tasksTotal++
		logInfo("task_started", task.ID, task.Description)

		contextFiles := resolveContextFiles(task)
		context := gatherFileContext(contextFiles)

		diff := executeTask(task, context)

		if err := validatePatch(diff, contextFiles, task); err != nil {

			if strings.Contains(err.Error(), "too large") {
				logInfo("patch_too_large_retrying", task.ID, err.Error())
				task.RetryCount++
				stats.totalRetries++

				if task.RetryCount < maxRetries {
					continue
				}

				logInfo("splitting_due_to_size", task.ID, "")
				splitTask(&tf, task)
				saveTasks(tf)
				continue
			}

			logError("patch_rejected", task.ID, err.Error())
			markBlocked(task)
			stats.tasksBlocked++
			saveTasks(tf)
			continue
		}

		if !dryRun {
			if err := applyPatch(diff); err != nil {
				logError("patch_apply_failed", task.ID, err.Error())
				markBlocked(task)
				stats.tasksBlocked++
				saveTasks(tf)
				continue
			}
		}

		buildOut := build()

		if buildOut == "" {
			completeTask(task)
			stats.recordSuccessfulPatch(diff)
			stats.tasksCompleted++
			saveTasks(tf)
			continue
		}
		stats.recordBuildFailure(buildOut)

		for task.RetryCount < maxRetries {
			task.RetryCount++
			stats.totalRetries++
			logInfo("fix_attempt", task.ID, fmt.Sprintf("retry %d", task.RetryCount))

			diff = fixTask(task, context, buildOut)

			if err := validatePatch(diff, contextFiles, task); err != nil {
				break
			}

			if !dryRun {
				if err := applyPatch(diff); err != nil {
					break
				}
			}

			buildOut = build()
			if buildOut == "" {
				completeTask(task)
				stats.recordSuccessfulPatch(diff)
				stats.tasksCompleted++
				saveTasks(tf)
				goto next
			}
			stats.recordBuildFailure(buildOut)
		}

		logInfo("task_splitting", task.ID, "max retries exceeded")
		splitTask(&tf, task)
		saveTasks(tf)

	next:
	}
}

func (s *executionStats) recordSuccessfulPatch(diff string) {
	s.largestPatch = max(s.largestPatch, lineCount(diff))
	for _, file := range filesTouched(diff) {
		s.modifiedFiles[file]++
	}
}

func (s *executionStats) recordBuildFailure(buildOut string) {
	failure := classifyBuildFailure(buildOut)
	if failure == "" {
		return
	}
	s.failurePatterns[failure]++
}
