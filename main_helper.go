package main

import (
	"encoding/json"
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
	// Execution flags
	modelName       string
	llmEndpoint     string
	maxRetries      int
	maxPatchLines   int
	maxFilesTouched int
	maxRuntime      time.Duration
	maxTasks        int
	resumeBranch    bool
	dryRun          bool
	verbose         bool
	selfEvolve      bool

	// Audit flags
	auditMode    bool
	auditPattern string
	auditPass    string
	auditOutput  string
)

const (
	defaultModel    = "local-27b"
	defaultEndpoint = "http://localhost:11434/v1/chat/completions"
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
// FLAGS
////////////////////////////////////////////////////////////

func parseFlags() {
	flag.StringVar(&modelName, "model", defaultModel, "LLM model name")
	flag.StringVar(&llmEndpoint, "endpoint", defaultEndpoint, "LLM endpoint URL")
	flag.IntVar(&maxRetries, "max-retries", 5, "Maximum retries per task")
	flag.IntVar(&maxPatchLines, "max-patch-lines", 50, "Maximum allowed patch size")
	flag.IntVar(&maxFilesTouched, "max-files", 3, "Maximum files touched per patch")
	flag.DurationVar(&maxRuntime, "max-runtime", 0, "Maximum runtime (e.g., 2h, 30m)")
	flag.IntVar(&maxTasks, "max-tasks", 0, "Maximum tasks per run")
	flag.BoolVar(&resumeBranch, "resume", false, "Resume current branch")
	flag.BoolVar(&dryRun, "dry-run", false, "Do not apply patches or commit")
	flag.BoolVar(&verbose, "verbose", false, "Print logs to stdout")
	flag.BoolVar(&selfEvolve, "self-evolve", false, "Enable elevated mutation limits for orchestrator self-improvement")
	flag.BoolVar(&auditMode, "audit", false, "Enable static analysis mode")
	flag.StringVar(&auditPattern, "audit-pattern", "./...", "Go package pattern to analyse")
	flag.StringVar(&auditPass, "audit-pass", "all", "One of architecture, api, concurrency, or all")
	flag.StringVar(&auditOutput, "audit-output", "audit_findings.json", "Output file for findings")

	flag.Usage = func() {
		fmt.Println("Autonomous Engineering Orchestrator")
		fmt.Println("\nUsage:")
		fmt.Println("  orchestrator [options]")
		flag.PrintDefaults()
	}

	flag.Parse()
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

	f, _ := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	defer f.Close()
	f.Write(append(b, '\n'))
}

func logInfo(event, taskID, msg string)  { log("INFO", event, taskID, msg) }
func logError(event, taskID, msg string) { log("ERROR", event, taskID, msg) }

////////////////////////////////////////////////////////////
// (Other previously defined helper functions unchanged)
////////////////////////////////////////////////////////////
