package main

import (
	"fmt"
	"math"
)

// executionStrategy defines a named approach to task execution.
type executionStrategy struct {
	name        string
	promptHint  string
	temperature float64
}

// strategies are the competing design approaches tried in Tier2Architectural mode.
var executionStrategies = []executionStrategy{
	{"conservative", "Make the smallest, most targeted change possible. Avoid any unnecessary churn.", 0.3},
	{"standard", "", 0.5},
	{"structural", "Prefer restructuring to minimise long-term complexity, even if the diff is slightly larger.", 0.7},
}

// strategyResult holds one strategy's output and composite evaluation score.
type strategyResult struct {
	strategy string
	diff     string
	score    float64
}

// strategyCompete runs all named strategies concurrently and returns the
// diff that scores highest on the composite evaluation function.
// Only called when activeTier >= Tier2Architectural.
func strategyCompete(task *Task, basePrompt string) string {
	results := make(chan strategyResult, len(executionStrategies))
	for _, s := range executionStrategies {
		go func(s executionStrategy) {
			prompt := basePrompt
			if s.promptHint != "" {
				prompt += "\n\nSTRATEGY HINT: " + s.promptHint
			}
			diff := callLLMWithModel(prompt, s.temperature, activeExecutorModel())
			score := evaluateStrategyDiff(diff, task)
			logInfo("strategy_evaluated", task.ID,
				fmt.Sprintf("strategy=%s score=%.3f lines=%d", s.name, score, lineCount(diff)))
			results <- strategyResult{s.name, diff, score}
		}(s)
	}

	best := strategyResult{score: -1}
	for range executionStrategies {
		r := <-results
		if r.score > best.score && r.diff != "" {
			best = r
		}
	}

	if best.diff != "" {
		logInfo("strategy_selected", task.ID,
			fmt.Sprintf("strategy=%s score=%.3f", best.strategy, best.score))
	}
	return best.diff
}

// evaluateStrategyDiff scores a candidate diff in [0, 1].
// Weights: low risk (40 %), low churn (35 %), size fitness (15 %), low entropy (10 %).
// Higher score = more desirable candidate.
func evaluateStrategyDiff(diff string, task *Task) float64 {
	if diff == "" {
		return -1
	}

	risk := scorePatchRisk(diff, task).score
	churn := structuralChurnScore(diff)
	entropy := entropyScore(diff)

	// Size fitness: penalise both under-use and over-use of the patch budget.
	allowed := float64(allowedPatchLines(task))
	lines := float64(lineCount(diff))
	sizeFit := clamp01(1 - math.Abs(lines/allowed-0.6))

	return clamp01((1-risk)*0.40 + (1-churn)*0.35 + sizeFit*0.15 + (1-entropy)*0.10)
}
