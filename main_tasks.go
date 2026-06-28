package main

import (
	"encoding/json"
	"fmt"
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

	resp := callLLM(prompt)

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

////////////////////////////////////////////////////////////
// EXECUTION
////////////////////////////////////////////////////////////

func executeTask(task *Task, context string) string {
	prompt := fmt.Sprintf(`
Implement task:
%s

Context:
%s

Return unified diff only.
`, task.Description, context)

	return callLLM(prompt)
}

func fixTask(task *Task, context, errors string) string {
	prompt := fmt.Sprintf(`
Fix errors:
%s

Task:
%s

Context:
%s

Return unified diff only.
`, errors, task.Description, context)

	return callLLM(prompt)
}
