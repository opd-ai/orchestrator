package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
)

var missingImportPattern = regexp.MustCompile(`undefined:\s+[A-Za-z_]\w*\.`)

func buildFixHints(buildOut string) string {
	errorLines := compilerErrorLines(buildOut)
	if len(errorLines) == 0 {
		return "Compiler issues: none detected."
	}

	var b strings.Builder
	b.WriteString("Compiler issues:\n")
	for _, line := range errorLines {
		category := classifyBuildFailure(line)
		fmt.Fprintf(&b, "- category: %s\n  error: %s\n  hint: %s\n", category, line, fixHintForCategory(category))
	}
	return strings.TrimSpace(b.String())
}

func compilerErrorLines(buildOut string) []string {
	lines := strings.Split(buildOut, "\n")
	var out []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
		if len(out) == 5 {
			break
		}
	}
	return out
}

func fixHintForCategory(category string) string {
	switch category {
	case "missing import":
		return "Add the missing import and ensure it is referenced with the correct package name."
	case "undefined symbol":
		return "Define the symbol or update references to an existing identifier in scope."
	case "unused import":
		return "Remove unused imports and keep the import block gofmt-compatible."
	case "type mismatch":
		return "Align argument and return types to the function and interface expectations."
	case "redeclaration":
		return "Remove duplicate declarations and keep one canonical identifier per scope."
	default:
		return "Apply a minimal patch that resolves this compiler error without unrelated refactors."
	}
}

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
		best, bestCount = pickHigherCount(best, bestCount, file, count)
	}

	return best
}

func mostCommonFailure(failures map[string]int) string {
	var best string
	bestCount := 0

	for failure, count := range failures {
		best, bestCount = pickHigherCount(best, bestCount, failure, count)
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
		case strings.Contains(line, "missing import"):
			return "missing import"
		case missingImportPattern.MatchString(line):
			return "missing import"
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

func pickHigherCount(current string, currentCount int, candidate string, candidateCount int) (string, int) {
	if candidateCount > currentCount {
		return candidate, candidateCount
	}
	if candidateCount == currentCount && candidateCount > 0 {
		candidates := []string{current, candidate}
		slices.Sort(candidates)
		return candidates[0], currentCount
	}
	return current, currentCount
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

// roleModel returns roleVar if it is non-empty, otherwise falls back to modelName.
// Used to implement model role specialization (--planner-model, --executor-model, etc.).
func roleModel(roleVar string) string {
	if roleVar != "" {
		return roleVar
	}
	return modelName
}
