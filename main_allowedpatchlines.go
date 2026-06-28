package main

import "strings"

func lineLimit(in int) int {
	if in > 360 {
		return 360
	}
	return in
}

func allowedPatchLines(task *Task) int {
	base := maxPatchLines

	// Elevated mode for orchestrator self-modification
	if selfEvolve && strings.Contains(strings.ToLower(task.Description), "orchestrator") {
		base = 150
	}

	// Adaptive escalation by retry count
	switch task.RetryCount {
	case 0:
		return lineLimit(base)
	case 1:
		return lineLimit(int(float64(base) * 1.4))
	case 2:
		return lineLimit(int(float64(base) * 1.8))
	default:
		return lineLimit(int(float64(base) * 2.5))
	}
}
