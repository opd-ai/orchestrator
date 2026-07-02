package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/opd-ai/orchestrator/memory"
)

type executionStats struct {
	tasksTotal           int
	tasksCompleted       int
	tasksBlocked         int
	totalRetries         int
	largestPatch         int
	highRiskPatches      int
	modifiedFiles        map[string]int
	failurePatterns      map[string]int
	convergenceSamples   int
	convergenceAlerts    int
	stability            stabilityMonitor
	subsystems           map[string]*subsystemMetrics
	totalPatchConfidence float64
	totalPatchRisk       float64
	successfulPatches    int
}

const architectRetryTemp = 0.8

// loopAction signals how the execute() loop should proceed after an iteration step.
type loopAction int

const (
	actionContinue loopAction = iota // proceed to next iteration
	actionDone                       // terminate the loop and return stats
	actionSkip                       // skip to next iteration (task was re-queued)
)

// newExecutionStats returns a zero-valued executionStats with all maps initialised.
func newExecutionStats() executionStats {
	return executionStats{
		modifiedFiles:   make(map[string]int),
		failurePatterns: make(map[string]int),
		subsystems:      make(map[string]*subsystemMetrics),
	}
}

type fileSnapshot struct {
	existed bool
	mode    os.FileMode
	data    []byte
}

// runExecutionMode restores state, executes pending tasks, and persists end-of-run memory summaries.
func runExecutionMode() {
	start := time.Now()
	atomic.StoreInt64(&inferenceLatencyTotal, 0)
	atomic.StoreInt64(&inferenceCallCount, 0)
	if err := recoverExecutionJournal(); err != nil {
		logError("journal_recovery_failed", "", err.Error())
	}
	if !resumeBranch {
		ensureBranch()
	}
	ensureTasksFile()

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
		HighRiskPatches:         stats.highRiskPatches,
		MostModifiedFile:        mostModifiedFile(stats.modifiedFiles),
		MostCommonFailure:       mostCommonFailure(stats.failurePatterns),
		RetryConvergenceSamples: stats.convergenceSamples,
		RetryConvergenceAlerts:  stats.convergenceAlerts,
		FailurePatterns:         copyCounts(stats.failurePatterns),
		ModifiedFiles:           copyCounts(stats.modifiedFiles),
		AvgInferenceLatencyMs:   averageInferenceLatency(),
		AvgPatchConfidence:      averageFloat(stats.totalPatchConfidence, stats.successfulPatches),
		AvgPatchRisk:            averageFloat(stats.totalPatchRisk, stats.successfulPatches),
	}

	if err := memory.SaveRun(summary); err != nil {
		logError("memory_save_failed", "", err.Error())
	}
	memory.UpdateMetrics(summary)
	writeRunSummary(summary)
}

// copyCounts returns a shallow copy of a string-to-count map.
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

// checkRunLimits returns true when the configured runtime or task-count limit has
// been reached, logging the cause and subsystem stats before signalling a stop.
func checkRunLimits(start time.Time, taskCounter int, stats *executionStats) bool {
	if maxRuntime > 0 && time.Since(start) > maxRuntime {
		logInfo("max_runtime_reached", "", "")
		logSubsystemStats(stats.subsystems)
		return true
	}
	if maxTasks > 0 && taskCounter >= maxTasks {
		logInfo("max_tasks_reached", "", "")
		logSubsystemStats(stats.subsystems)
		return true
	}
	return false
}

// advanceTaskFile loads the task file, runs cluster merges, and returns the
// next executable task together with a loopAction directing the caller.
// actionDone means all tasks are complete; actionSkip means the task was
// re-queued (caller should continue); actionContinue means task is ready.
func advanceTaskFile(stats *executionStats) (TaskFile, *Task, loopAction) {
	tf := loadTasks()
	if mergeClusteredTasks(&tf) {
		saveTasks(tf)
	}
	task := nextExecutableTask(&tf)
	if task == nil {
		logInfo("run_complete", "", "All tasks complete")
		logSubsystemStats(stats.subsystems)
		return tf, nil, actionDone
	}
	if task.MergedCount <= 1 && enforceTaskGranularity(&tf, task) {
		logInfo("task_split_pre_execution", task.ID, "deterministic granularity enforcer")
		saveTasks(tf)
		return tf, nil, actionSkip
	}
	return tf, task, actionContinue
}

// blockTask marks task as blocked, records the failure in stats, and persists tf.
func blockTask(tf *TaskFile, task *Task, stats *executionStats) {
	markBlocked(task)
	stats.tasksBlocked++
	stats.stability.recordBlock()
	recordSubsystemOutcome(stats.subsystems, task, false)
	saveTasks(*tf)
}

// setupTaskEnv logs the task start, de-escalates/re-escalates models and tiers,
// and ensures the workspace is clean. Returns false if the workspace could not
// be cleaned (task is blocked and caller should continue to next iteration).
func setupTaskEnv(tf *TaskFile, task *Task, stats *executionStats) bool {
	stats.tasksTotal++
	logInfo("task_started", task.ID, task.Description)
	deescalateTier(task.ID)
	deescalateModel(task.ID)
	deescalateReviewMode()
	maybeEscalateTier(task, stats)
	maybeEscalateModel(task, scorePatchRisk("", task), stats.tasksTotal)
	if err := ensureCleanWorkspace(task.ID); err != nil {
		logError("workspace_reset_failed", task.ID, err.Error())
		blockTask(tf, task, stats)
		return false
	}
	return true
}

// gatherAndValidateDiff resolves context, generates a diff via the LLM, and
// validates it. On validation failure the task is either retried, split, or
// blocked. Returns (diff, context, contextFiles, ok); ok==false means the
// caller should continue to the next iteration.
func gatherAndValidateDiff(tf *TaskFile, task *Task, taskCache map[string]string, stats *executionStats) (string, string, []string, bool) {
	contextFiles := resolveContextFiles(task)
	context := gatherContextForTask(task, contextFiles)
	diff := getDiffForTask(task, context, taskCache, stats)
	if err := validatePatch(diff, contextFiles, task); err == nil {
		return diff, context, contextFiles, true
	} else if strings.Contains(err.Error(), "too large") {
		writeRejectedPatch(task.ID, diff)
		logInfo("patch_too_large_retrying", task.ID, err.Error())
		task.RetryCount++
		stats.totalRetries++
		if task.RetryCount < maxRetries {
			saveTasks(*tf) // persist incremented RetryCount so splitting threshold advances across iterations
			return diff, context, contextFiles, false
		}
		logInfo("splitting_due_to_size", task.ID, "")
		splitTask(tf, task)
		saveTasks(*tf)
	} else {
		writeRejectedPatch(task.ID, diff)
		logError("patch_rejected", task.ID, err.Error())
		blockTask(tf, task, stats)
	}
	return diff, context, contextFiles, false
}

// applyPatchStep applies the diff to the workspace and records the journal entry.
// Returns false (and blocks the task) if the apply fails.
func applyPatchStep(tf *TaskFile, task *Task, diff string, stats *executionStats) bool {
	if err := applyDiffToWorkspace(diff, task); err != nil {
		logError("patch_apply_failed", task.ID, err.Error())
		blockTask(tf, task, stats)
		return false
	}
	if err := recordExecutionJournal(task.ID, journalStepPatched, diff); err != nil {
		logError("journal_write_failed", task.ID, err.Error())
	}
	return true
}

// runBuildStep skips the build in dry-run mode (returning "") or runs build()
// and returns its output for failure analysis.
func runBuildStep(taskID string) string {
	if dryRun {
		logInfo("dry_run_build_skipped", taskID, "")
		return ""
	}
	return build()
}

// handleBuildResult processes the outcome of a build attempt: on success it
// completes the task and updates stats; on failure it delegates to
// resolveBuildFailure.
func handleBuildResult(tf *TaskFile, task *Task, diff, context string, contextFiles []string, buildOut string, stats *executionStats, taskCache map[string]string) {
	if buildOut != "" {
		if !dryRun {
			resolveBuildFailure(tf, task, context, contextFiles, diff, buildOut, stats, taskCache)
		}
		return
	}
	if dryRun {
		logInfo("dry_run_task_ready", task.ID, "patch_generated=true patch_valid=true would_apply=true")
	} else if err := recordExecutionJournal(task.ID, journalStepBuilt, diff); err != nil {
		logError("journal_write_failed", task.ID, err.Error())
	}
	completeTask(task)
	stats.recordSuccessfulPatch(diff, task)
	stats.tasksCompleted++
	recordSubsystemOutcome(stats.subsystems, task, true)
	recordSubsystemPatchMetrics(task, diff)
	cacheTaskResult(taskCache, task, diff)
	saveTaskCache(taskCache)
	saveTasks(*tf)
}

// execute runs the main task-execution loop, returning a summary of all work
// performed in the session.
func execute() executionStats {
	start := time.Now()
	taskCounter := 0
	stats := newExecutionStats()
	taskCache := loadTaskCache()
	for {
		if checkRunLimits(start, taskCounter, &stats) {
			return stats
		}
		tf, task, action := advanceTaskFile(&stats)
		if action == actionDone {
			return stats
		}
		if action == actionSkip {
			continue
		}
		taskCounter++
		if !setupTaskEnv(&tf, task, &stats) {
			continue
		}
		diff, context, contextFiles, ok := gatherAndValidateDiff(&tf, task, taskCache, &stats)
		if !ok {
			continue
		}
		if !dryRun && !applyPatchStep(&tf, task, diff, &stats) {
			continue
		}
		buildOut := runBuildStep(task.ID)
		handleBuildResult(&tf, task, diff, context, contextFiles, buildOut, &stats, taskCache)
	}
}

// getDiffForTask returns the diff for a task, using the cache when available
// and falling back to strategic review or normal execution.
func getDiffForTask(task *Task, context string, taskCache map[string]string, stats *executionStats) string {
	if cached := cachedDiff(taskCache, task); cached != "" {
		logInfo("task_cache_hit", task.ID, "using cached diff")
		return cached
	}
	if shouldTriggerStrategicReview(task, stats) {
		return executeInReviewMode(task, stats)
	}
	return executeTask(task, context)
}

// applyDiffToWorkspace applies a diff to the working tree and then validates
// post-patch architectural invariants, reverting on violation.
func applyDiffToWorkspace(diff string, task *Task) error {
	if err := applyPatch(diff); err != nil {
		return err
	}
	return checkPostPatchInvariants(diff, filesTouched(diff), task)
}

// resolveBuildFailure records a failed build, attempts local fixes, and splits the task if retries are exhausted.
func resolveBuildFailure(
	tf *TaskFile,
	task *Task,
	context string,
	contextFiles []string,
	originalDiff string,
	buildOut string,
	stats *executionStats,
	taskCache map[string]string,
) {
	stats.recordBuildFailure(buildOut)
	previousFailure := classifyBuildFailure(buildOut)
	writeBuildFailure(task.ID, buildOut, task.RetryCount)
	buildOut, trivialFixSnapshots := tryTrivialFixes(tf, task, originalDiff, buildOut, stats, taskCache)
	if buildOut == "" {
		return
	}

	appliedFixDiffs, resolved := attemptBuildFixRetries(
		tf, task, context, contextFiles, buildOut, previousFailure, stats, taskCache,
	)
	if resolved {
		return
	}

	if !dryRun {
		if err := revertBuildFailurePatches(originalDiff, appliedFixDiffs, trivialFixSnapshots); err != nil {
			logError("patch_revert_failed", task.ID, err.Error())
		}
	}
	logInfo("task_splitting", task.ID, "max retries exceeded")
	splitTask(tf, task)
	saveTasks(*tf)
}

// attemptBuildFixRetries iterates FIX prompts until the build passes or the retry budget is spent.
func attemptBuildFixRetries(
	tf *TaskFile,
	task *Task,
	context string,
	contextFiles []string,
	buildOut string,
	previousFailure string,
	stats *executionStats,
	taskCache map[string]string,
) ([]string, bool) {
	appliedFixDiffs := make([]string, 0, maxRetries)
	forceArchitectRetry := false
	for task.RetryCount < maxRetries {
		task.RetryCount++
		stats.totalRetries++
		logInfo("fix_attempt", task.ID, fmt.Sprintf("retry %d", task.RetryCount))

		previousDiff := previousRetryDiff(appliedFixDiffs)
		temperature, model := fixRetrySettings(task.RetryCount, forceArchitectRetry)
		fixDiff := fixTask(task, context, fixTaskConfig{
			hints:        buildFixHints(buildOut),
			previousDiff: previousDiff,
			temperature:  temperature,
			model:        model,
		})
		if err := validatePatch(fixDiff, contextFiles, task); err != nil {
			// Validation happens before append/apply, so a rejected fix diff is not
			// included in appliedFixDiffs.
			writeRejectedPatch(task.ID, fixDiff)
			return appliedFixDiffs, false
		}
		if !dryRun {
			appliedFixDiffs = append(appliedFixDiffs, fixDiff)
			if err := applyPatch(fixDiff); err != nil {
				return appliedFixDiffs, false
			}
		}

		buildOut = build()
		if buildOut == "" {
			if !dryRun {
				if err := recordExecutionJournal(task.ID, journalStepBuilt, fixDiff); err != nil {
					logError("journal_write_failed", task.ID, err.Error())
				}
			}
			completeTask(task)
			stats.recordSuccessfulPatch(fixDiff, task)
			stats.tasksCompleted++
			cacheTaskResult(taskCache, task, fixDiff)
			saveTaskCache(taskCache)
			saveTasks(*tf)
			return appliedFixDiffs, true
		}
		stats.recordBuildFailure(buildOut)
		currentFailure := classifyBuildFailure(buildOut)
		forceArchitectRetry = stats.recordRetryConvergence(task.ID, task.RetryCount, previousFailure, currentFailure)
		previousFailure = currentFailure
		writeBuildFailure(task.ID, buildOut, task.RetryCount)
	}

	return appliedFixDiffs, false
}

// revertBuildFailurePatches rolls back the original patch, applied fix diffs, and any trivial file edits.
func revertBuildFailurePatches(originalDiff string, appliedFixDiffs []string, trivialFixSnapshots map[string]fileSnapshot) error {
	var revertErrors []string
	// Best-effort rollback: keep attempting all reverts so we restore as much state
	// as possible and report every failure to operators in one error.
	for i := len(appliedFixDiffs) - 1; i >= 0; i-- {
		if err := revertPatch(appliedFixDiffs[i]); err != nil {
			revertErrors = append(revertErrors, fmt.Sprintf("fix patch %d: %v", len(appliedFixDiffs)-i, err))
		}
	}
	if err := restoreFileSnapshots(trivialFixSnapshots); err != nil {
		revertErrors = append(revertErrors, fmt.Sprintf("trivial fixes: %v", err))
	}
	if err := revertPatch(originalDiff); err != nil {
		revertErrors = append(revertErrors, fmt.Sprintf("original patch: %v", err))
	}
	if len(revertErrors) > 0 {
		return fmt.Errorf("revert failed: %s", strings.Join(revertErrors, "; "))
	}
	return nil
}

// tryTrivialFixes applies deterministic source edits before falling back to another LLM retry.
func tryTrivialFixes(
	tf *TaskFile,
	task *Task,
	diff string,
	buildOut string,
	stats *executionStats,
	taskCache map[string]string,
) (string, map[string]fileSnapshot) {
	touchedFiles := goFilesFromContext(filesTouched(diff))
	if dryRun {
		return buildOut, nil
	}
	trivialFixSnapshots, err := snapshotFiles(touchedFiles)
	if err != nil {
		// Best-effort rollback: keep partial snapshots so we can still restore any
		// files we captured successfully.
		logError("trivial_fix_snapshot_failed", task.ID, err.Error())
	}
	if !applyTrivialFixes(touchedFiles, buildOut) {
		return buildOut, nil
	}

	logInfo("trivial_fix_attempted", task.ID, "")
	buildOut = build()
	if buildOut != "" {
		stats.recordBuildFailure(buildOut)
		writeBuildFailure(task.ID, buildOut, task.RetryCount)
		return buildOut, trivialFixSnapshots
	}
	if err := recordExecutionJournal(task.ID, journalStepBuilt, diff); err != nil {
		logError("journal_write_failed", task.ID, err.Error())
	}

	completeTask(task)
	stats.recordSuccessfulPatch(diff, task)
	stats.tasksCompleted++
	cacheTaskResult(taskCache, task, diff)
	saveTaskCache(taskCache)
	saveTasks(*tf)
	return "", nil
}

// snapshotFiles captures file contents and modes so trivial fixes can be reverted safely.
func snapshotFiles(paths []string) (map[string]fileSnapshot, error) {
	snapshots := make(map[string]fileSnapshot, len(paths))
	var snapshotErrors []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				snapshots[path] = fileSnapshot{}
				continue
			}
			snapshotErrors = append(snapshotErrors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			snapshotErrors = append(snapshotErrors, fmt.Sprintf("%s: %v", path, err))
			continue
		}
		snapshots[path] = fileSnapshot{
			existed: true,
			mode:    info.Mode().Perm(),
			data:    data,
		}
	}
	if len(snapshotErrors) > 0 {
		return snapshots, fmt.Errorf("snapshot failed: %s", strings.Join(snapshotErrors, "; "))
	}
	return snapshots, nil
}

// restoreFileSnapshots restores files to the captured snapshot state.
func restoreFileSnapshots(snapshots map[string]fileSnapshot) error {
	if len(snapshots) == 0 {
		return nil
	}
	var restoreErrors []string
	for path, snapshot := range snapshots {
		if !snapshot.existed {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				restoreErrors = append(restoreErrors, fmt.Sprintf("%s: %v", path, err))
			}
			continue
		}
		if err := os.WriteFile(path, snapshot.data, snapshot.mode); err != nil {
			restoreErrors = append(restoreErrors, fmt.Sprintf("%s: %v", path, err))
		}
	}
	if len(restoreErrors) > 0 {
		return fmt.Errorf("restore failed: %s", strings.Join(restoreErrors, "; "))
	}
	return nil
}

// ensureCleanWorkspace resets tracked and untracked changes before starting a task when resets are enabled.
func ensureCleanWorkspace(taskID string) error {
	if dryRun || skipWorkspaceReset {
		return nil
	}
	dirty, err := workspaceDirty()
	if err != nil {
		return err
	}
	if !dirty {
		return nil
	}

	logInfo("dirty_workspace_detected", taskID, "")
	logInfo("dirty_workspace_resetting", taskID, "resetting tracked and untracked files")
	if out, err := exec.Command("git", "reset", "--hard", "HEAD").CombinedOutput(); err != nil {
		return fmt.Errorf("failed to reset workspace (git reset --hard HEAD): %w (%s)", err, sanitizeCommandOutput(out))
	}
	cleanArgs := []string{
		"clean", "-fd",
		"--exclude=" + tasksFile,
		"--exclude=" + logFile,
		"--exclude=" + journalFile,
	}
	if out, err := exec.Command("git", cleanArgs...).CombinedOutput(); err != nil {
		return fmt.Errorf("failed to clean untracked workspace files (git clean): %w (%s)", err, sanitizeCommandOutput(out))
	}
	return nil
}

// workspaceDirty reports whether tracked or staged changes are present in the repository.
func workspaceDirty() (bool, error) {
	dirty, err := commandMarksDirty("git", "diff", "--quiet", "HEAD")
	if err != nil {
		return false, err
	}
	if dirty {
		return true, nil
	}
	return commandMarksDirty("git", "diff", "--cached", "--quiet")
}

// commandMarksDirty treats exit code 1 as a dirty-state signal for git diff-style commands.
func commandMarksDirty(name string, args ...string) (bool, error) {
	cmd := exec.Command(name, args...)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

// sanitizeCommandOutput flattens command output for single-line log messages.
func sanitizeCommandOutput(out []byte) string {
	clean := strings.TrimSpace(string(out))
	clean = strings.ReplaceAll(clean, "\n", " | ")
	if clean == "" {
		return "no command output"
	}
	return clean
}

// recordSuccessfulPatch updates execution metrics after a patch completes successfully.
func (s *executionStats) recordSuccessfulPatch(diff string, task *Task) {
	patchSize := lineCount(diff)
	patchRisk := scorePatchRisk(diff, task)
	s.largestPatch = max(s.largestPatch, patchSize)
	for _, file := range filesTouched(diff) {
		s.modifiedFiles[file]++
	}
	s.trackHighRisk(patchRisk.level)
	computeReward(task.ID, task.RetryCount, patchSize)
	s.stability.recordSuccess()
	s.totalPatchConfidence += evaluatePatchConfidence(diff).score
	s.totalPatchRisk += patchRisk.score
	s.successfulPatches++
}

// trackHighRisk increments the high-risk counter when a patch crosses the configured risk threshold.
func (s *executionStats) trackHighRisk(level RiskLevel) {
	if level >= RiskHigh {
		s.highRiskPatches++
	}
}

// recordBuildFailure buckets a build error by failure category for run summaries.
func (s *executionStats) recordBuildFailure(buildOut string) {
	failure := classifyBuildFailure(buildOut)
	if failure == "" {
		return
	}
	s.failurePatterns[failure]++
}

// fixRetrySettings returns the temperature and model to use for the next fix attempt, forcing the architect model when requested.
func fixRetrySettings(retryCount int, forceArchitect bool) (float64, string) {
	if forceArchitect {
		if architectModelName != "" {
			return architectRetryTemp, architectModelName
		}
		return architectRetryTemp, modelName
	}
	return tempForRetry(retryCount), activeExecutorModel()
}

// previousRetryDiff returns the most recently applied fix diff, or an empty string when no prior retry exists.
func previousRetryDiff(appliedFixDiffs []string) string {
	if len(appliedFixDiffs) == 0 {
		return ""
	}
	return appliedFixDiffs[len(appliedFixDiffs)-1]
}

// recordRetryConvergence tracks repeated failure categories across consecutive fix attempts.
// It returns true only when the next retry should force architect mode, and false
// for low retry counts, empty failure categories, or category changes between retries.
func (s *executionStats) recordRetryConvergence(taskID string, retryCount int, previous, current string) bool {
	if retryCount < 2 || current == "" {
		return false
	}

	s.convergenceSamples++
	if previous != current {
		return false
	}

	s.convergenceAlerts++
	s.stability.recordOscillation()
	logInfo(
		"retry_convergence_alert",
		taskID,
		fmt.Sprintf("retry %d repeated failure %q", retryCount, current),
	)
	return true
}
