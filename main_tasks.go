package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	oversizedTaskDescriptionLimit = 180
	oversizedTaskConjunctionLimit = 2
	previousAttemptLineLimit      = 20
)

////////////////////////////////////////////////////////////
// TASK SPLITTING
////////////////////////////////////////////////////////////

// splitTask asks the planner model to decompose a blocked task into smaller atomic subtasks.
func splitTask(tf *TaskFile, task *Task) {
	prompt := fmt.Sprintf(`
Split into smaller atomic tasks.
Return JSON array only.

Task:
%s
`, task.Description)

	resp := callLLMWithModel(promptWithMemory(prompt), 0.6, roleModel(architectModelName))

	clean, err := extractJSON(resp)
	if err != nil {
		logError("split_failed", task.ID, err.Error())
		task.Status = "blocked"
		return
	}

	var subtasks []Task
	if err := json.Unmarshal([]byte(clean), &subtasks); err != nil {
		logError("split_failed", task.ID, err.Error())
		task.Status = "blocked"
		return
	}

	prefix := task.ID + "."
	for i := range subtasks {
		subtasks[i].ID = fmt.Sprintf("%s%d", prefix, i+1)
		subtasks[i].Status = "pending"
		subtasks[i].DependsOn = task.DependsOn
	}

	replaceTask(tf, task.ID, subtasks)
}

// replaceTask swaps one task ID for a new set of replacement tasks.
func replaceTask(tf *TaskFile, id string, newTasks []Task) {
	var updated []Task
	for _, t := range tf.Tasks {
		if t.ID != id {
			updated = append(updated, t)
		}
	}
	updated = append(updated, newTasks...)
	tf.Tasks = updated
}

// enforceTaskGranularity splits broad tasks before execution when deterministic rules say they are too large.
func enforceTaskGranularity(tf *TaskFile, task *Task) bool {
	if len(task.Files) > 1 {
		replaceTask(tf, task.ID, splitMultiFileTask(task))
		return true
	}
	if splitBySymbols(tf, task) {
		return true
	}
	if !isOversizedTask(task.Description) {
		return false
	}
	subtasks := splitOversizedDescription(task)
	if len(subtasks) < 2 {
		return false
	}
	replaceTask(tf, task.ID, subtasks)
	return true
}

// splitBySymbols tries to decompose a single-file task into symbol-scoped subtasks.
// It returns true when the task was replaced by ≥2 symbol subtasks.
func splitBySymbols(tf *TaskFile, task *Task) bool {
	if len(task.Files) != 1 || isAlreadySymbolTask(task.ID) {
		return false
	}
	subtasks := symbolTasksForFiles(task.ID, task.Files)
	if len(subtasks) < 2 {
		return false
	}
	replaceTask(tf, task.ID, subtasks)
	return true
}

// symbolTaskRe matches the ".s<digits>" suffix produced by generateSymbolTask,
// e.g. "T1.s3" or "R2.s12".
var symbolTaskRe = regexp.MustCompile(`\.s\d+`)

// isAlreadySymbolTask reports whether a task ID was produced by symbolTasksForFiles,
// i.e. it contains the ".s<digits>" suffix used by generateSymbolTask.
func isAlreadySymbolTask(id string) bool {
	return symbolTaskRe.MatchString(id)
}

// splitMultiFileTask turns a multi-file task into one pending subtask per file.
func splitMultiFileTask(task *Task) []Task {
	prefix := task.ID + "."
	subtasks := make([]Task, 0, len(task.Files))
	for i, file := range task.Files {
		subtasks = append(subtasks, Task{
			ID:          fmt.Sprintf("%s%d", prefix, i+1),
			Description: fmt.Sprintf("%s (%s)", task.Description, file),
			Files:       []string{file},
			DependsOn:   task.DependsOn,
			Status:      "pending",
		})
	}
	return subtasks
}

// isOversizedTask reports whether a task description should be decomposed before execution.
func isOversizedTask(description string) bool {
	return len(description) > oversizedTaskDescriptionLimit ||
		strings.Count(description, " and ") >= oversizedTaskConjunctionLimit
}

// splitOversizedDescription breaks a long description into smaller pending subtasks.
func splitOversizedDescription(task *Task) []Task {
	parts := regexp.MustCompile(`\s*(?:;|,|\band\b)\s*`).Split(task.Description, -1)
	prefix := task.ID + "."
	subtasks := make([]Task, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if len(part) < 8 {
			continue
		}
		subtasks = append(subtasks, Task{
			ID:          fmt.Sprintf("%s%d", prefix, len(subtasks)+1),
			Description: part,
			Files:       append([]string(nil), task.Files...),
			DependsOn:   task.DependsOn,
			Status:      "pending",
		})
	}
	return subtasks
}

////////////////////////////////////////////////////////////
// EXECUTION
////////////////////////////////////////////////////////////

// executeTask builds the execution prompt and dispatches it through the active execution strategy.
func executeTask(task *Task, context string) string {
	prompt := promptWithMemory(buildExecPrompt(task, context))
	return dispatchExecution(task, context, prompt)
}

// dispatchExecution selects the appropriate execution path based on active mode and tier.
func dispatchExecution(task *Task, context, prompt string) string {
	switch {
	case speculativeMode:
		return speculativeExecute(task, context)
	case activeTier >= Tier2Architectural:
		return strategyCompete(task, prompt)
	default:
		return callLLMWithModel(prompt, 0.6, activeExecutorModel())
	}
}

// buildExecPrompt assembles the unified-diff prompt for the main execution path.
func buildExecPrompt(task *Task, context string) string {
	constraints := []string{
		"Modify only what is strictly necessary",
		"Do not refactor unrelated code",
		"Keep patch minimal and atomic",
		"Follow strict unified diff format",
		"Do not include markdown fences",
	}
	return fmt.Sprintf(`
%s

Task:
%s

Context:
%s

Return unified diff only.
`, executionBlock("EXECUTE", task, constraints, ""), task.Description, context)
}

// fixTask builds a retry prompt that asks the model to correct a failed patch.
func fixTask(task *Task, context, hints, previousDiff string, temperature float64, model string) string {
	prompt := buildFixPrompt(task, context, hints, previousDiff)
	return callLLMWithModel(promptWithMemory(prompt), temperature, model)
}

// buildFixPrompt assembles the retry prompt, including a preview of the previous failed diff when available.
func buildFixPrompt(task *Task, context, hints, previousDiff string) string {
	constraints := []string{
		"Return a corrected unified diff",
		"Keep patch minimal and atomic",
		"Do not rewrite large blocks",
	}
	prompt := fmt.Sprintf(`
%s

Task:
%s

Context:
%s

%s

Return unified diff only.
`, executionBlock("FIX", task, constraints, hints), task.Description, context, previousAttemptBlock(previousDiff))
	return strings.TrimSpace(prompt)
}

// executionBlock formats the structured execution metadata injected into EXECUTE and FIX prompts.
func executionBlock(mode string, task *Task, constraints []string, failReason string) string {
	var b strings.Builder
	b.WriteString("EXECUTION_BLOCK\n")
	b.WriteString("MODE: " + mode + "\n")
	b.WriteString("TASK_ID: " + task.ID + "\n")
	b.WriteString("FILES_ALLOWED: " + strings.Join(task.Files, ",") + "\n")
	b.WriteString(fmt.Sprintf("MAX_PATCH_LINES: %d\n", allowedPatchLines(task)))
	b.WriteString(fmt.Sprintf("MAX_FILE_PATCH_LINES: %d\n", perFileLineDeltaCap(task)))
	writeOptionalFields(&b, task, constraints, failReason)
	return strings.TrimSpace(b.String())
}

// writeOptionalFields appends optional execution metadata to a prompt block.
func writeOptionalFields(b *strings.Builder, task *Task, constraints []string, failReason string) {
	if task.ChangeType != "" {
		b.WriteString("CHANGE_TYPE: " + string(task.ChangeType) + "\n")
	}
	b.WriteString("CONSTRAINTS:\n")
	for _, c := range constraints {
		b.WriteString("- " + c + "\n")
	}
	if failReason != "" {
		b.WriteString("FAIL_REASON:\n")
		b.WriteString(failReason + "\n")
	}
}

// tempForRetry returns the retry temperature profile for the FIX loop.
func tempForRetry(retryCount int) float64 {
	switch retryCount {
	case 1:
		return 0.3
	case 2:
		return 0.7
	default:
		return 0.5
	}
}

// previousAttemptBlock formats the capped previous diff preview for FIX prompts.
func previousAttemptBlock(previousDiff string) string {
	preview := firstDiffLines(previousDiff, previousAttemptLineLimit)
	if preview == "" {
		return ""
	}
	return "PREVIOUS_ATTEMPT (failed):\n" + preview
}

// firstDiffLines returns the first limit lines from a diff, or an empty string when no preview is available.
func firstDiffLines(diff string, limit int) string {
	if limit <= 0 {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(diff), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return ""
	}
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return strings.Join(lines, "\n")
}
