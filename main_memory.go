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
	return compressPrompt(plannerMemoryContext + "\n\n" + prompt)
}

func compressPrompt(prompt string) string {
	lines := strings.Split(prompt, "\n")
	out := make([]string, 0, len(lines))
	last := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == last {
			continue
		}
		out = append(out, line)
		last = line
	}
	compressed := strings.Join(out, "\n")
	if len(compressed) <= promptCharBudget {
		return compressed
	}
	return compressed[:promptCharBudget]
}
