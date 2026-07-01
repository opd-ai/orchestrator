package main

import (
	"fmt"
	"strings"
)

// RiskLevel classifies patch mutation risk.
type RiskLevel int

const (
	RiskLow    RiskLevel = 0
	RiskMedium RiskLevel = 1
	RiskHigh   RiskLevel = 2
)

// riskHighThreshold and riskGateThreshold define the scoring thresholds.
// Patches exceeding riskGateThreshold on a first-attempt (RetryCount==0) are
// rejected to force a more targeted retry.
const (
	riskHighThreshold = 0.65
	riskGateThreshold = 0.85
)

// patchRisk holds the risk breakdown for a single patch.
type patchRisk struct {
	score            float64
	level            RiskLevel
	deletionFactor   float64
	centralityFactor float64
	retryFactor      float64
	apiSurfaceFactor float64
}

// scorePatchRisk computes a weighted risk score for a patch in [0, 1].
// Weights: centrality 35 %, API surface 25 %, deletion ratio 20 %, retry pressure 20 %.
func scorePatchRisk(diff string, task *Task) patchRisk {
	del := deletionRatio(diff)
	api := apiSurfaceScore(diff)
	retry := clamp01(float64(task.RetryCount) / float64(maxRetries+1))
	central := touchedFileCentrality(filesTouched(diff))

	score := clamp01((0.35 * central) + (0.25 * api) + (0.20 * del) + (0.20 * retry))

	level := RiskLow
	switch {
	case score > riskHighThreshold:
		level = RiskHigh
	case score > 0.35:
		level = RiskMedium
	}

	return patchRisk{
		score:            score,
		level:            level,
		deletionFactor:   del,
		centralityFactor: central,
		retryFactor:      retry,
		apiSurfaceFactor: api,
	}
}

func (r patchRisk) message() string {
	return fmt.Sprintf(
		"risk=%.2f level=%s deletion=%.2f centrality=%.2f retry=%.2f api=%.2f",
		r.score, r.levelString(), r.deletionFactor, r.centralityFactor, r.retryFactor, r.apiSurfaceFactor,
	)
}

func (r patchRisk) levelString() string {
	switch r.level {
	case RiskHigh:
		return "high"
	case RiskMedium:
		return "medium"
	default:
		return "low"
	}
}

// apiSurfaceScore returns [0,1] based on the fraction of added lines that
// declare exported Go symbols (func, type, interface).
func apiSurfaceScore(diff string) float64 {
	apiLines := 0
	totalAdded := 0
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		totalAdded++
		trimmed := strings.TrimSpace(line[1:])
		for _, keyword := range []string{"func ", "type ", "interface "} {
			if !strings.HasPrefix(trimmed, keyword) {
				continue
			}
			after := strings.TrimSpace(trimmed[len(keyword):])
			if len(after) > 0 && after[0] >= 'A' && after[0] <= 'Z' {
				apiLines++
			}
		}
	}
	if totalAdded == 0 {
		return 0
	}
	// Amplify: even 10 % API-surface lines signal meaningful risk.
	return clamp01(float64(apiLines) / float64(totalAdded) * 10)
}

// touchedFileCentrality returns [0,1] representing how central the modified
// files are. Root-package files score 1.0; sub-package files score 0.3.
func touchedFileCentrality(files []string) float64 {
	if len(files) == 0 {
		return 0
	}
	total := 0.0
	for _, f := range files {
		if strings.Contains(f, "/") {
			total += 0.3
		} else {
			total += 1.0
		}
	}
	return clamp01(total / float64(len(files)))
}

// validatePatchRisk gates very-high-risk patches on first attempts.
// If a task has already been retried, the gating is skipped to avoid infinite loops.
func validatePatchRisk(diff string, task *Task) error {
	if task.RetryCount > 0 {
		return nil
	}
	r := scorePatchRisk(diff, task)
	logInfo("patch_risk", task.ID, r.message())
	if r.score > riskGateThreshold {
		return fmt.Errorf("patch risk too high on first attempt (score=%.2f)", r.score)
	}
	return nil
}
