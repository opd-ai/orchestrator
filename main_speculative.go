package main

import "sync"

// speculativeResult holds the outcome of one parallel LLM candidate attempt.
type speculativeResult struct {
	diff  string
	score float64
}

// tempStrategy returns the ordered temperature values for speculative execution.
// The first value is low (deterministic); the second is moderate (exploratory).
func tempStrategy() []float64 { return []float64{0.3, 0.6} }

// speculativeExecute generates candidate diffs at multiple temperatures in parallel,
// scores each by patch confidence and retry penalty, and returns the best diff.
// The number of parallel candidates adapts to current CPU load via speculativeCandidateCount.
// It is concurrency-safe: all goroutines write to distinct result slots and the
// score computation is pure/stateless.
func speculativeExecute(task *Task, context string) string {
	temps := tempStrategy()
	n := speculativeCandidateCount()
	if n < len(temps) {
		temps = temps[:n]
	}
	results := make([]speculativeResult, len(temps))

	var wg sync.WaitGroup
	for i, temp := range temps {
		i, temp := i, temp
		wg.Add(1)
		go func() {
			defer wg.Done()
			prompt := promptWithMemory(buildExecPrompt(task, context))
			diff := callLLMWithTemp(prompt, temp)
			results[i] = speculativeResult{
				diff:  diff,
				score: speculativeScore(diff, task.RetryCount),
			}
		}()
	}
	wg.Wait()

	return pickBestResult(results)
}

// speculativeScore computes a composite quality score for a candidate diff.
// It combines patch confidence (entropy, deletion ratio, structural churn)
// with a small penalty for high retry counts to prefer fresh approaches.
func speculativeScore(diff string, retryCount int) float64 {
	confidence := evaluatePatchConfidence(diff)
	retryPenalty := float64(retryCount) * 0.05
	return clamp01(confidence.score - retryPenalty)
}

// pickBestResult returns the diff from the highest-scoring candidate.
func pickBestResult(results []speculativeResult) string {
	if len(results) == 0 {
		return ""
	}
	best := results[0]
	for _, r := range results[1:] {
		if r.score > best.score {
			best = r
		}
	}
	return best.diff
}
