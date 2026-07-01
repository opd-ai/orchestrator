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
		FailurePatterns:   copyCounts(stats.failurePatterns),
		ModifiedFiles:     copyCounts(stats.modifiedFiles),
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

		diff := executeTask(task, context)

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
		resolveBuildFailure(&tf, task, context, contextFiles, diff, buildOut, &stats)
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
) {
	stats.recordBuildFailure(buildOut)
	writeBuildFailure(task.ID, buildOut)
	buildOut = tryTrivialFixes(tf, task, diff, buildOut, stats)
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
			stats.recordSuccessfulPatch(diff)
			stats.tasksCompleted++
			saveTasks(*tf)
			return
		}
		stats.recordBuildFailure(buildOut)
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
	stats.recordSuccessfulPatch(diff)
	stats.tasksCompleted++
	saveTasks(*tf)
	return ""
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
