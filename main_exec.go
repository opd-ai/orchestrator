package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/opd-ai/orchestrator/memory"
)

func runExecutionMode() {
	start := time.Now()

	// Inject memory into planner
	memoryContext := memory.SummarizeForPlanner()
	injectMemoryIntoPlanner(memoryContext)

	fmt.Println("Execution mode started.")
	fmt.Println("Model:", modelName)
	execute()

	// Save memory summary at end
	summary := memory.RunSummary{
		Timestamp:       time.Now(),
		Branch:          currentGitBranch(),
		DurationSeconds: int64(time.Since(start).Seconds()),
	}

	memory.SaveRun(summary)
	memory.UpdateMetrics(summary)
}

func execute() {
	parseFlags()

	start := time.Now()
	taskCounter := 0

	if !resumeBranch {
		ensureBranch()
	}

	ensureTasksFile()

	for {
		if maxRuntime > 0 && time.Since(start) > maxRuntime {
			logInfo("max_runtime_reached", "", "")
			return
		}

		if maxTasks > 0 && taskCounter >= maxTasks {
			logInfo("max_tasks_reached", "", "")
			return
		}

		tf := loadTasks()
		task := nextExecutableTask(&tf)
		if task == nil {
			logInfo("run_complete", "", "All tasks complete")
			return
		}

		taskCounter++
		logInfo("task_started", task.ID, task.Description)

		contextFiles := resolveContextFiles(task)
		context := gatherFileContext(contextFiles)

		diff := executeTask(task, context)

		if err := validatePatch(diff, contextFiles, task); err != nil {

			if strings.Contains(err.Error(), "too large") {
				logInfo("patch_too_large_retrying", task.ID, err.Error())
				task.RetryCount++

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
			saveTasks(tf)
			continue
		}

		if !dryRun {
			if err := applyPatch(diff); err != nil {
				logError("patch_apply_failed", task.ID, err.Error())
				markBlocked(task)
				saveTasks(tf)
				continue
			}
		}

		buildOut := build()

		if buildOut == "" {
			completeTask(task)
			saveTasks(tf)
			continue
		}

		for task.RetryCount < maxRetries {
			task.RetryCount++
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
				saveTasks(tf)
				goto next
			}
		}

		logInfo("task_splitting", task.ID, "max retries exceeded")
		splitTask(&tf, task)
		saveTasks(tf)

	next:
	}
}
