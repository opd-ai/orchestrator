package main

import "fmt"

const modelEscalationCooldownTasks = 5

// modelEscalationState tracks whether the executor is currently using an
// escalated (larger) model and when the last escalation occurred.
type modelEscalationState struct {
	active               bool
	lastEscalationAtTask int // stats.tasksTotal value at last escalation
}

// activeModelEscalation is the package-level escalation state. It is reset to
// the zero value at process start and updated during execute().
var activeModelEscalation modelEscalationState

// maybeEscalateModel promotes the executor model when escalation conditions
// are met and the cooldown period has elapsed. Escalation requires that
// --escalate-model is non-empty.
//
// Escalation conditions (any one is sufficient):
//   - task.RetryCount >= 2 (persistent failure on this task)
//   - risk level is High
//   - activeTier >= Tier2Architectural
func maybeEscalateModel(task *Task, risk patchRisk, tasksTotal int) {
	if escalateModelName == "" || activeModelEscalation.active {
		return
	}
	cooldownOK := tasksTotal-activeModelEscalation.lastEscalationAtTask >= modelEscalationCooldownTasks
	if !cooldownOK {
		return
	}
	if task.RetryCount < 2 && risk.level < RiskHigh && activeTier < Tier2Architectural {
		return
	}
	activeModelEscalation.active = true
	activeModelEscalation.lastEscalationAtTask = tasksTotal
	logInfo("model_escalated", task.ID, fmt.Sprintf(
		"model=%s retries=%d risk=%s tier=%d",
		escalateModelName, task.RetryCount, risk.levelString(), activeTier,
	))
}

// deescalateModel reverts to the standard executor model after a task finishes.
func deescalateModel(taskID string) {
	if activeModelEscalation.active {
		logInfo("model_deescalated", taskID, fmt.Sprintf("reverted from %s", escalateModelName))
		activeModelEscalation.active = false
	}
}

// activeExecutorModel returns the model name to use for task execution,
// reflecting any active escalation.
func activeExecutorModel() string {
	if activeModelEscalation.active && escalateModelName != "" {
		return escalateModelName
	}
	return roleModel(executorModelName)
}
