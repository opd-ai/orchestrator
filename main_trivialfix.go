package main

import (
	"os/exec"
	"path/filepath"
	"slices"
)

func applyTrivialFixes(contextFiles []string, buildOut string) bool {
	goFiles := goFilesFromContext(contextFiles)
	if len(goFiles) == 0 {
		return false
	}

	ranFix := false
	if shouldRunGoimports(classifyCompilerIssues(buildOut)) {
		if runGoimports(goFiles) == nil {
			ranFix = true
		}
	}
	if runGoFmt(goFiles) == nil {
		ranFix = true
	}

	return ranFix
}

func goFilesFromContext(contextFiles []string) []string {
	var out []string
	for _, file := range contextFiles {
		if filepath.Ext(file) == ".go" {
			out = append(out, file)
		}
	}
	return out
}

func classifyCompilerIssues(buildOut string) []string {
	set := map[string]bool{}
	for _, line := range compilerErrorLines(buildOut) {
		set[classifyBuildFailure(line)] = true
	}

	issues := make([]string, 0, len(set))
	for issue := range set {
		issues = append(issues, issue)
	}
	slices.Sort(issues)
	return issues
}

func shouldRunGoimports(issues []string) bool {
	for _, issue := range issues {
		if issue == "missing import" || issue == "unused import" {
			return true
		}
	}
	return false
}

func runGoFmt(goFiles []string) error {
	args := append([]string{"-w"}, goFiles...)
	return exec.Command("gofmt", args...).Run()
}

func runGoimports(goFiles []string) error {
	args := append([]string{"-w"}, goFiles...)
	if _, err := exec.LookPath("goimports"); err == nil {
		return exec.Command("goimports", args...).Run()
	}
	return exec.Command("go", append([]string{"run", "golang.org/x/tools/cmd/goimports@v0.47.0"}, args...)...).Run()
}
