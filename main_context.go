package main

import (
	"os"
	"strings"

	"github.com/opd-ai/orchestrator/audit"
)

// gatherContextForTask returns function-scoped context when a target function is
// identifiable from the task description, otherwise returns full file context.
func gatherContextForTask(task *Task, files []string) string {
	sm, err := audit.AnalyzeFiles(files)
	if err != nil {
		return gatherFileContext(files)
	}
	if name := matchSymbol(task.Description, sm); name != "" {
		if ctx := funcScopedContext(name, sm); ctx != "" {
			return ctx
		}
	}
	return gatherFileContext(files)
}

// matchSymbol returns the first function name from sm that appears in desc,
// enabling function-level context restriction.
func matchSymbol(desc string, sm *audit.SymbolMap) string {
	lower := strings.ToLower(desc)
	for name := range sm.Functions {
		if strings.Contains(lower, strings.ToLower(name)) {
			return name
		}
	}
	return ""
}

// funcScopedContext extracts the source lines of the named function.
func funcScopedContext(name string, sm *audit.SymbolMap) string {
	fbs, ok := sm.Functions[name]
	if !ok || len(fbs) == 0 {
		return ""
	}
	return extractBoundaryContext(fbs[0])
}

// extractBoundaryContext returns the source lines for the given FuncBoundary.
func extractBoundaryContext(fb audit.FuncBoundary) string {
	lines, err := readLines(fb.File)
	if err != nil {
		return ""
	}
	start := max(0, fb.StartLine-1)
	end := min(len(lines), fb.EndLine)
	return "FILE: " + fb.File + "\n" + strings.Join(lines[start:end], "\n")
}

// readLines reads a file and splits it into lines.
func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}
