package main

import "strings"

var plannerMemoryContext string

func injectMemoryIntoPlanner(memoryContext string) {
	plannerMemoryContext = strings.TrimSpace(memoryContext)
	if plannerMemoryContext == "" {
		return
	}
	logInfo("memory_injected", "", "Adaptive metrics injected into planner")
}

func promptWithMemory(prompt string) string {
	if plannerMemoryContext == "" {
		return prompt
	}
	return plannerMemoryContext + "\n\n" + prompt
}
