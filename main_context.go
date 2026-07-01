package main

import (
	"os"
	"sort"
	"strings"

	"github.com/opd-ai/orchestrator/audit"
)

// gatherContextForTask returns function-scoped context when a target function is
// identifiable from the task description, otherwise returns full file context.
func gatherContextForTask(task *Task, files []string) string {
	sm, _ := audit.AnalyzeFiles(files)
	if len(sm.Functions) == 0 && len(sm.Structs) == 0 {
		return gatherFileContext(files)
	}
	if key := matchSymbol(task.Description, sm); key != "" {
		if ctx := funcScopedContext(key, sm, task.Files); ctx != "" {
			return ctx
		}
	}
	return gatherFileContext(files)
}

// matchSymbol returns the SymbolMap key of the best-matching function from sm
// for the given description. It iterates keys in sorted order and prefers the
// longest whole-word match to minimise false positives and ensure determinism.
func matchSymbol(desc string, sm *audit.SymbolMap) string {
	lower := strings.ToLower(desc)

	keys := make([]string, 0, len(sm.Functions))
	for k := range sm.Functions {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var bestKey string
	var bestNameLen int
	for _, key := range keys {
		fbs := sm.Functions[key]
		if len(fbs) == 0 {
			continue
		}
		name := fbs[0].Name
		if len(name) < 4 {
			// Skip very short names to reduce false positives.
			continue
		}
		if !containsWord(lower, strings.ToLower(name)) {
			continue
		}
		if len(name) > bestNameLen {
			bestKey = key
			bestNameLen = len(name)
		}
	}
	return bestKey
}

// funcScopedContext extracts the source lines of the named function (looked up by key).
// If multiple boundaries exist, the one whose file is listed in taskFiles is preferred;
// when no single boundary can be chosen unambiguously the function returns "" so the
// caller falls back to full-file context.
func funcScopedContext(key string, sm *audit.SymbolMap, taskFiles []string) string {
	fbs, ok := sm.Functions[key]
	if !ok || len(fbs) == 0 {
		return ""
	}
	if len(fbs) == 1 {
		return extractBoundaryContext(fbs[0])
	}
	// Multiple boundaries: prefer one whose file is explicitly in taskFiles.
	for _, fb := range fbs {
		for _, tf := range taskFiles {
			if fb.File == tf {
				return extractBoundaryContext(fb)
			}
		}
	}
	// Ambiguous and no file hint — fall back to full-file context.
	return ""
}

// containsWord reports whether text contains word as a whole identifier token
// (case-insensitive). Adjacent identifier characters (letters, digits, underscore)
// on either side disqualify the match.
func containsWord(text, word string) bool {
	start := 0
	for {
		idx := strings.Index(text[start:], word)
		if idx < 0 {
			return false
		}
		idx += start
		end := idx + len(word)
		leftOK := idx == 0 || !isIdentChar(text[idx-1])
		rightOK := end == len(text) || !isIdentChar(text[end])
		if leftOK && rightOK {
			return true
		}
		start = idx + 1
	}
}

func isIdentChar(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
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
