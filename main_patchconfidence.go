package main

import (
	"fmt"
	"math"
	"strings"
)

type patchConfidence struct {
	score           float64
	entropy         float64
	deletionRatio   float64
	structuralChurn float64
}

func evaluatePatchConfidence(diff string) patchConfidence {
	entropy := entropyScore(diff)
	delRatio := deletionRatio(diff)
	churn := structuralChurnScore(diff)

	score := 1 - (0.45 * entropy) - (0.35 * delRatio) - (0.20 * churn)
	return patchConfidence{
		score:           clamp01(score),
		entropy:         entropy,
		deletionRatio:   delRatio,
		structuralChurn: churn,
	}
}

func (p patchConfidence) message() string {
	return fmt.Sprintf(
		"score=%.2f entropy=%.2f deletion_ratio=%.2f structural_churn=%.2f",
		p.score, p.entropy, p.deletionRatio, p.structuralChurn,
	)
}

func entropyScore(diff string) float64 {
	changed := changedLines(diff)
	if len(changed) <= 1 {
		return 0
	}

	counts := make(map[string]int)
	for _, line := range changed {
		counts[line]++
	}

	total := float64(len(changed))
	entropy := 0.0
	for _, count := range counts {
		p := float64(count) / total
		entropy -= p * math.Log2(p)
	}

	maxEntropy := math.Log2(float64(len(counts)))
	if maxEntropy == 0 {
		return 0
	}
	return clamp01(entropy / maxEntropy)
}

func changedLines(diff string) []string {
	lines := strings.Split(diff, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "+"), strings.HasPrefix(line, "-"):
			out = append(out, strings.TrimSpace(line[1:]))
		}
	}
	return out
}

func structuralChurnScore(diff string) float64 {
	files := len(filesTouched(diff))
	hunks := 0

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "@@") {
			hunks++
		}
	}

	filePenalty := max(0, files-1)
	hunkPenalty := max(0, hunks-1)
	churn := (0.25 * float64(filePenalty)) + (0.10 * float64(hunkPenalty))
	return clamp01(churn)
}

func clamp01(in float64) float64 {
	if in < 0 {
		return 0
	}
	if in > 1 {
		return 1
	}
	return in
}
