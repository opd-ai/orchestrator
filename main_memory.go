package main

import (
	"strings"

	"github.com/opd-ai/orchestrator/audit"
)

var plannerMemoryContext string

const promptCharBudget = 6000

func injectMemoryIntoPlanner(memoryContext string) {
	plannerMemoryContext = strings.TrimSpace(memoryContext)
	if plannerMemoryContext == "" {
		return
	}
	logInfo("memory_injected", "", "Adaptive metrics injected into planner")
}

// injectInvariantSummary loads the invariant registry and appends its
// summary to plannerMemoryContext so every LLM prompt includes the constraints.
func injectInvariantSummary() {
	reg, err := audit.LoadInvariantRegistry()
	if err != nil {
		// Registry is optional; absence is not an error.
		return
	}
	summary := reg.Summary()
	if summary == "" {
		return
	}
	if plannerMemoryContext != "" {
		plannerMemoryContext = plannerMemoryContext + "\n" + summary
	} else {
		plannerMemoryContext = summary
	}
	logInfo("invariants_injected", "", "Architectural invariants injected into planner")
}

func promptWithMemory(prompt string) string {
	if plannerMemoryContext == "" {
		return compressPrompt(prompt)
	}
	return compressPrompt(plannerMemoryContext + "\n" + prompt)
}

func compressPrompt(prompt string) string {
	compressed := strings.TrimSpace(prompt)
	if len(compressed) <= promptCharBudget {
		return compressed
	}
	runes := []rune(compressed)
	if len(runes) <= promptCharBudget {
		return compressed
	}
	return string(runes[:promptCharBudget])
}
