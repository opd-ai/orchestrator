package main

import "strings"

var plannerMemoryContext string

const promptCharBudget = 6000

func injectMemoryIntoPlanner(memoryContext string) {
	plannerMemoryContext = strings.TrimSpace(memoryContext)
	if plannerMemoryContext == "" {
		return
	}
	logInfo("memory_injected", "", "Adaptive metrics injected into planner")
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
