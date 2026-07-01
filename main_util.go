package main

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

func currentGitBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func averageRetries(totalRetries, tasks int) float64 {
	if tasks == 0 {
		return 0
	}
	return float64(totalRetries) / float64(tasks)
}

func mostModifiedFile(files map[string]int) string {
	var best string
	bestCount := 0

	for file, count := range files {
		if count > bestCount {
			best = file
			bestCount = count
			continue
		}

		if count == bestCount && count > 0 {
			candidates := []string{best, file}
			slices.Sort(candidates)
			best = candidates[0]
		}
	}

	return best
}

func mostCommonFailure(failures map[string]int) string {
	var best string
	bestCount := 0

	for failure, count := range failures {
		if count > bestCount {
			best = failure
			bestCount = count
			continue
		}

		if count == bestCount && count > 0 {
			candidates := []string{best, failure}
			slices.Sort(candidates)
			best = candidates[0]
		}
	}

	return best
}

func classifyBuildFailure(buildOut string) string {
	lines := strings.Split(buildOut, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.Contains(line, "undefined:"):
			return "undefined symbol"
		case strings.Contains(line, "imported and not used"):
			return "unused import"
		case strings.Contains(line, "cannot use"):
			return "type mismatch"
		case strings.Contains(line, "redeclared in this block"):
			return "redeclaration"
		}
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}

	return ""
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
