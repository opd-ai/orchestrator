package main

import (
	"strings"
	"time"

	"github.com/opd-ai/orchestrator/memory"
)

func main() {
	parseFlags()

	start := time.Now()
	taskCounter := 0
	largestPatchSeen := 0
	mostModifiedFile := ""
	totalRetries := 0

	if !resumeBranch {
		ensureBranch()
	}

	ensureTasksFile()

	// Inject memory context into planner if bootstrapping
	memoryContext := memory.SummarizeForPlanner()
	injectMemoryIntoPlanner(memoryContext)

	for {
		if maxRuntime > 0 && time.Since(start) > maxRuntime {
			logInfo("max_runtime_reached", "", "")
			break
		}

		if maxTasks > 0 && taskCounter >= maxTasks {
			logInfo("max_tasks_reached", "", "")
			break
		}

		tf := loadTasks()
		task := nextExecutableTask(&tf)
		if task == nil {
			logInfo("run_complete", "", "All tasks complete")
			break
		}

		taskCounter++
		logInfo("task_started", task.ID, task.Description)

		contextFiles := resolveContextFiles(task)
		context := gatherFileContext(contextFiles)

		for {
			diff := executeTask(task, context)

			lines := lineCount(diff)
			if lines > largestPatchSeen {
				largestPatchSeen = lines
			}

			filesTouched := filesTouched(diff)
			if len(filesTouched) > 0 {
				mostModifiedFile = filesTouched[0]
			}

			err := validatePatch(diff, contextFiles, task)
			if err != nil {
				if strings.Contains(err.Error(), "too large") {
					logInfo("patch_too_large_retrying", task.ID, err.Error())
					task.RetryCount++
					totalRetries++

					if task.RetryCount < maxRetries {
						continue
					}

					logInfo("splitting_due_to_size", task.ID, "")
					splitTask(&tf, task)
					saveTasks(tf)
					goto nextTask
				}

				logError("patch_rejected", task.ID, err.Error())
				markBlocked(task)
				saveTasks(tf)
				goto nextTask
			}

			if !dryRun {
				if err := applyPatch(diff); err != nil {
					logError("patch_apply_failed", task.ID, err.Error())
					task.RetryCount++
					totalRetries++

					if task.RetryCount >= maxRetries {
						splitTask(&tf, task)
						saveTasks(tf)
						goto nextTask
					}
					continue
				}
			}

			buildOutput := build()
			if buildOutput != "" {
				task.RetryCount++
				totalRetries++

				if task.RetryCount >= maxRetries {
					splitTask(&tf, task)
					saveTasks(tf)
					goto nextTask
				}

				diff = fixTask(task, context, buildOutput)
				continue
			}

			completeTask(task)
			saveTasks(tf)
			break
		}

	nextTask:
	}

	duration := time.Since(start)

	// Save memory summary
	summary := memory.RunSummary{
		Timestamp:        time.Now(),
		Branch:           currentGitBranch(),
		DurationSeconds:  int64(duration.Seconds()),
		TasksTotal:       taskCounter,
		TasksCompleted:   countCompleted(),
		TasksBlocked:     countBlocked(),
		AvgRetries:       averageRetries(totalRetries, taskCounter),
		LargestPatch:     largestPatchSeen,
		MostModifiedFile: mostModifiedFile,
	}

	memory.SaveRun(summary)
	memory.UpdateMetrics(summary)
}
