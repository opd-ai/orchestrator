package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/opd-ai/orchestrator/memory"
)

type executionStats struct {
	tasksTotal         int
	tasksCompleted     int
	tasksBlocked       int
	totalRetries       int
	largestPatch       int
	modifiedFiles      map[string]int
	failurePatterns    map[string]int
	convergenceSamples int
	convergenceAlerts  int
	stability          stabilityMonitor
}

func runExecutionMode() {
	start := time.Now()

	// Inject memory into planner
	memoryContext := memory.SummarizeForPlanner()
	injectMemoryIntoPlanner(memoryContext)
	injectInvariantSummary()

	fmt.Println("Execution mode started.")
	fmt.Println("Model:", modelName)
	stats := execute()

	// Save memory summary at end
	summary := memory.RunSummary{
		Timestamp:               time.Now(),
		Branch:                  currentGitBranch(),
		DurationSeconds:         int64(time.Since(start).Seconds()),
		TasksTotal:              stats.tasksTotal,
		TasksCompleted:          stats.tasksCompleted,
		TasksBlocked:            stats.tasksBlocked,
		AvgRetries:              averageRetries(stats.totalRetries, stats.tasksTotal),
		LargestPatch:            stats.largestPatch,
		MostModifiedFile:        mostModifiedFile(stats.modifiedFiles),
		MostCommonFailure:       mostCommonFailure(stats.failurePatterns),
		RetryConvergenceSamples: stats.convergenceSamples,
		RetryConvergenceAlerts:  stats.convergenceAlerts,
		FailurePatterns:         copyCounts(stats.failurePatterns),
		ModifiedFiles:           copyCounts(stats.modifiedFiles),
	}

	memory.SaveRun(summary)
	memory.UpdateMetrics(summary)
	writeRunSummary(summary)
}

func copyCounts(in map[string]int) map[string]int {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func execute() executionStats {
	start := time.Now()
	taskCounter := 0
	stats := executionStats{
		modifiedFiles:   make(map[string]int),
		failurePatterns: make(map[string]int),
	}
	taskCache := loadTaskCache()

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
		if enforceTaskGranularity(&tf, task) {
			logInfo("task_split_pre_execution", task.ID, "deterministic granularity enforcer")
			saveTasks(tf)
			continue
		}

		taskCounter++
		stats.tasksTotal++
		logInfo("task_started", task.ID, task.Description)

		contextFiles := resolveContextFiles(task)
		context := gatherContextForTask(task, contextFiles)

		// Use cached diff if available to avoid an unnecessary LLM call.
		diff := cachedDiff(taskCache, task)
		if diff == "" {
			diff = executeTask(task, context)
		} else {
			logInfo("task_cache_hit", task.ID, "using cached diff")
		}

		if err := validatePatch(diff, contextFiles, task); err != nil {
			writeRejectedPatch(task.ID, diff)

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
			stats.stability.recordBlock()
			saveTasks(tf)
			continue
		}

		if !dryRun {
			if err := applyPatch(diff); err != nil {
				logError("patch_apply_failed", task.ID, err.Error())
				markBlocked(task)
				stats.tasksBlocked++
				stats.stability.recordBlock()
				saveTasks(tf)
				continue
			}
		}

		buildOut := build()

		if buildOut == "" {
			completeTask(task)
			stats.recordSuccessfulPatch(diff, task)
			stats.tasksCompleted++
			cacheTaskResult(taskCache, task, diff)
			saveTaskCache(taskCache)
			saveTasks(tf)
			continue
		}
		resolveBuildFailure(&tf, task, context, contextFiles, diff, buildOut, &stats, taskCache)
	}
}

func resolveBuildFailure(
	tf *TaskFile,
	task *Task,
	context string,
	contextFiles []string,
	diff string,
	buildOut string,
	stats *executionStats,
	taskCache map[string]string,
) {
	stats.recordBuildFailure(buildOut)
	previousFailure := classifyBuildFailure(buildOut)
	writeBuildFailure(task.ID, buildOut)
	buildOut = tryTrivialFixes(tf, task, diff, buildOut, stats, taskCache)
	if buildOut == "" {
		return
	}

	for task.RetryCount < maxRetries {
		task.RetryCount++
		stats.totalRetries++
		logInfo("fix_attempt", task.ID, fmt.Sprintf("retry %d", task.RetryCount))

		diff = fixTask(task, context, buildFixHints(buildOut))
		if err := validatePatch(diff, contextFiles, task); err != nil {
			writeRejectedPatch(task.ID, diff)
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
			stats.recordSuccessfulPatch(diff, task)
			stats.tasksCompleted++
			cacheTaskResult(taskCache, task, diff)
			saveTaskCache(taskCache)
			saveTasks(*tf)
			return
		}
		stats.recordBuildFailure(buildOut)
		currentFailure := classifyBuildFailure(buildOut)
		stats.recordRetryConvergence(task.ID, task.RetryCount, previousFailure, currentFailure)
		previousFailure = currentFailure
		writeBuildFailure(task.ID, buildOut)
	}

	logInfo("task_splitting", task.ID, "max retries exceeded")
	splitTask(tf, task)
	saveTasks(*tf)
}

func tryTrivialFixes(
	tf *TaskFile,
	task *Task,
	diff string,
	buildOut string,
	stats *executionStats,
	taskCache map[string]string,
) string {
	touchedFiles := goFilesFromContext(filesTouched(diff))
	if dryRun || !applyTrivialFixes(touchedFiles, buildOut) {
		return buildOut
	}

	logInfo("trivial_fix_attempted", task.ID, "")
	buildOut = build()
	if buildOut != "" {
		stats.recordBuildFailure(buildOut)
		writeBuildFailure(task.ID, buildOut)
		return buildOut
	}

	completeTask(task)
	stats.recordSuccessfulPatch(diff, task)
	stats.tasksCompleted++
	cacheTaskResult(taskCache, task, diff)
	saveTaskCache(taskCache)
	saveTasks(*tf)
	return ""
}

func (s *executionStats) recordSuccessfulPatch(diff string, task *Task) {
	patchSize := lineCount(diff)
	s.largestPatch = max(s.largestPatch, patchSize)
	for _, file := range filesTouched(diff) {
		s.modifiedFiles[file]++
	}
	computeReward(task.ID, task.RetryCount, patchSize)
	s.stability.recordSuccess()
}

func (s *executionStats) recordBuildFailure(buildOut string) {
	failure := classifyBuildFailure(buildOut)
	if failure == "" {
		return
	}
	s.failurePatterns[failure]++
}

func (s *executionStats) recordRetryConvergence(taskID string, retryCount int, previous, current string) {
	if retryCount < 2 || current == "" {
		return
	}

	s.convergenceSamples++
	if previous != current {
		return
	}

	s.convergenceAlerts++
	s.stability.recordOscillation()
	logInfo(
		"retry_convergence_alert",
		taskID,
		fmt.Sprintf("retry %d repeated failure %q", retryCount, current),
	)
}
