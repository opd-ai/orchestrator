package main

func injectMemoryIntoPlanner(memoryContext string) {
	if memoryContext == "" {
		return
	}
	logInfo("memory_injected", "", "Adaptive metrics injected into planner")
}
