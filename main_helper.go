package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"
)

////////////////////////////////////////////////////////////
// CONFIGURATION (via flags)
////////////////////////////////////////////////////////////

var (
	modelName       string
	llmEndpoint     string
	maxRetries      int
	maxPatchLines   int
	maxFilesTouched int
	refactorMode    bool
	maxRuntime      time.Duration
	maxTasks        int
	resumeBranch    bool
	dryRun          bool
	verbose         bool
)

const (
	defaultModel    = "local-27b"
	defaultEndpoint = "http://localhost:8000/v1/chat/completions"
	logFile         = "orchestrator.log"
	tasksFile       = "tasks.json"
	maxContextFiles = 5
)

////////////////////////////////////////////////////////////
// TYPES
////////////////////////////////////////////////////////////

type Task struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Files       []string `json:"files,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
	Status      string   `json:"status"`
	RetryCount  int      `json:"retry_count"`
	Hash        string   `json:"hash"`
}

type TaskFile struct {
	Tasks []Task `json:"tasks"`
}

type LogEntry struct {
	Timestamp string `json:"ts"`
	Level     string `json:"level"`
	Event     string `json:"event"`
	TaskID    string `json:"task_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

////////////////////////////////////////////////////////////
// MAIN
////////////////////////////////////////////////////////////

func main() {
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

		if err := validatePatch(diff, contextFiles); err != nil {
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

			if err := validatePatch(diff, contextFiles); err != nil {
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

////////////////////////////////////////////////////////////
// FLAGS
////////////////////////////////////////////////////////////

func parseFlags() {
	flag.StringVar(&modelName, "model", defaultModel, "LLM model name")
	flag.StringVar(&llmEndpoint, "endpoint", defaultEndpoint, "LLM endpoint URL")
	flag.IntVar(&maxRetries, "max-retries", 5, "Maximum retries per task")
	flag.IntVar(&maxPatchLines, "max-patch-lines", 50, "Maximum allowed patch size")
	flag.IntVar(&maxFilesTouched, "max-files", 3, "Maximum files touched per patch")
	flag.BoolVar(&refactorMode, "refactor", false, "Enable refactor mode")
	flag.DurationVar(&maxRuntime, "max-runtime", 0, "Maximum runtime (e.g., 2h, 30m)")
	flag.IntVar(&maxTasks, "max-tasks", 0, "Maximum tasks per run")
	flag.BoolVar(&resumeBranch, "resume", false, "Resume current branch")
	flag.BoolVar(&dryRun, "dry-run", false, "Do not apply patches or commit")
	flag.BoolVar(&verbose, "verbose", false, "Print logs to stdout")

	flag.Usage = func() {
		fmt.Println("Autonomous Engineering Orchestrator")
		fmt.Println("\nUsage:")
		fmt.Println("  orchestrator [options]\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if refactorMode {
		maxPatchLines = 120
	}
}

////////////////////////////////////////////////////////////
// (ALL PREVIOUS CORE LOGIC REMAINS SAME)
// For brevity below, only modified/critical helpers included.
////////////////////////////////////////////////////////////

func ensureBranch() {
	branch := fmt.Sprintf("autonomous/%d", time.Now().Unix())
	exec.Command("git", "checkout", "-b", branch).Run()
	logInfo("branch_created", "", branch)
}

func validatePatch(diff string, allowed []string) error {
	if lineCount(diff) > maxPatchLines {
		return errors.New("patch too large")
	}
	if len(filesTouched(diff)) > maxFilesTouched {
		return errors.New("too many files modified")
	}
	return nil
}

func completeTask(task *Task) {
	if !dryRun {
		gitCommit(task)
	}
	task.Status = "complete"
	logInfo("task_complete", task.ID, "")
}

func log(level, event, taskID, msg string) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     level,
		Event:     event,
		TaskID:    taskID,
		Message:   msg,
	}
	b, _ := json.Marshal(entry)

	if verbose {
		fmt.Println(string(b))
	}

	f, _ := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	f.Write(append(b, '\n'))
}

func logInfo(event, taskID, msg string)  { log("INFO", event, taskID, msg) }
func logError(event, taskID, msg string) { log("ERROR", event, taskID, msg) }

////////////////////////////////////////////////////////////
// (Other previously defined helper functions unchanged)
////////////////////////////////////////////////////////////
