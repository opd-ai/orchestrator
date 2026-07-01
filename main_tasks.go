package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

////////////////////////////////////////////////////////////
// TASK SPLITTING
////////////////////////////////////////////////////////////

func splitTask(tf *TaskFile, task *Task) {
	prompt := fmt.Sprintf(`
Split into smaller atomic tasks.
Return JSON array only.

Task:
%s
`, task.Description)

	resp := callLLM(promptWithMemory(prompt))

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

func enforceTaskGranularity(tf *TaskFile, task *Task) bool {
	if len(task.Files) > 1 {
		replaceTask(tf, task.ID, splitMultiFileTask(task))
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

func isOversizedTask(description string) bool {
	return len(description) > 180 || strings.Count(description, " and ") >= 2
}

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
			Files:       task.Files,
			DependsOn:   task.DependsOn,
			Status:      "pending",
		})
	}
	return subtasks
}

////////////////////////////////////////////////////////////
// EXECUTION
////////////////////////////////////////////////////////////

func executeTask(task *Task, context string) string {
	constraints := []string{
		"Modify only what is strictly necessary",
		"Do not refactor unrelated code",
		"Keep patch minimal and atomic",
		"Follow strict unified diff format",
		"Do not include markdown fences",
	}
	prompt := fmt.Sprintf(`
%s

Context:
%s

Return unified diff only.
`, executionBlock("EXECUTE", task, constraints, ""), context)

	return callLLM(promptWithMemory(prompt))
}

func fixTask(task *Task, context, hints string) string {
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

Return unified diff only.
`, executionBlock("FIX", task, constraints, hints), task.Description, context)
	return callLLM(promptWithMemory(prompt))
}

func executionBlock(mode string, task *Task, constraints []string, failReason string) string {
	var b strings.Builder
	b.WriteString("EXECUTION_BLOCK\n")
	b.WriteString("MODE: " + mode + "\n")
	b.WriteString("TASK_ID: " + task.ID + "\n")
	b.WriteString("FILES_ALLOWED: " + strings.Join(task.Files, ",") + "\n")
	b.WriteString(fmt.Sprintf("MAX_PATCH_LINES: %d\n", allowedPatchLines(task)))
	b.WriteString("CONSTRAINTS:\n")
	for _, constraint := range constraints {
		b.WriteString("- " + constraint + "\n")
	}
	if failReason != "" {
		b.WriteString("FAIL_REASON:\n")
		b.WriteString(failReason + "\n")
	}
	return strings.TrimSpace(b.String())
}
