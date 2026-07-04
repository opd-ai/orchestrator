package main

import (
	"os"
	"os/exec"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/opd-ai/orchestrator/audit"
	"github.com/opd-ai/orchestrator/memory"
)

var gitRecentFilesOutput = func() ([]byte, error) {
	return exec.Command("git", "log", "--since=7.days.ago", "--name-only", "--format=").Output()
}

var (
	cachedRecentFilesSet       map[string]bool
	cachedRecentFilesSetLoaded bool
)

// gatherContextForTask returns function-scoped context when a target function is
// identifiable from the task description, otherwise returns full file context.
// Parse errors from AnalyzeFiles are intentionally ignored: the partial SymbolMap
// returned for successfully-parsed files is still useful for context narrowing.
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
		if len(name) < minSymbolNameLen {
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
// caller falls back to full-file context. Results exceeding maxBytesPerFile are
// truncated at the byte boundary and marked with "// ... (truncated)".
func funcScopedContext(key string, sm *audit.SymbolMap, taskFiles []string) string {
	fbs, ok := sm.Functions[key]
	if !ok || len(fbs) == 0 {
		return ""
	}
	if len(fbs) == 1 {
		return truncateFuncContext(extractBoundaryContext(fbs[0]))
	}
	// Multiple boundaries: prefer one whose file is explicitly in taskFiles.
	for _, fb := range fbs {
		for _, tf := range taskFiles {
			if fb.File == tf {
				return truncateFuncContext(extractBoundaryContext(fb))
			}
		}
	}
	// Ambiguous and no file hint — fall back to full-file context.
	return ""
}

// truncateFuncContext caps a function body context string at maxBytesPerFile bytes.
// Truncation is performed on a rune boundary to avoid splitting multi-byte UTF-8
// sequences. When truncation occurs, a marker comment is appended so the model is aware.
func truncateFuncContext(ctx string) string {
	if len(ctx) <= maxBytesPerFile {
		return ctx
	}
	// Walk back from maxBytesPerFile until we land on the start of a rune.
	n := maxBytesPerFile
	for n > 0 && !utf8.RuneStart(ctx[n]) {
		n--
	}
	return ctx[:n] + "\n// ... (truncated)"
}

// minSymbolNameLen is the minimum character length a function name must have to be
// considered as a match candidate. Very short names (e.g. "id", "ok") produce too
// many false positives when searched as substrings of task descriptions.
const minSymbolNameLen = 4

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
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
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

// contextFileScore holds a file path and its computed relevance score.
type contextFileScore struct {
	path  string
	score int
}

// scoreContextFiles ranks candidates by weighted relevance and returns the top
// maxContextFiles entries. Files scoring zero are excluded. Ties are broken by
// lexical path order to ensure determinism across calls.
//
// Weights:
//   - keyword match:        5 (path contains a keyword from the task description)
//   - successful-edit history: 3 (file appears in the memories-branch patch history)
//   - git recency:          1 (file was modified within the last 7 days)
func scoreContextFiles(candidates []string, kw string, historySet, recentSet map[string]bool) []string {
	scored := make([]contextFileScore, 0, len(candidates))
	for _, path := range candidates {
		if s := contextFileWeight(path, kw, historySet, recentSet); s > 0 {
			scored = append(scored, contextFileScore{path, s})
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].path < scored[j].path
	})
	result := make([]string, 0, maxContextFiles)
	for _, c := range scored {
		if len(result) >= maxContextFiles {
			break
		}
		result = append(result, c.path)
	}
	if len(result) == 0 {
		return fallbackContextFiles(candidates)
	}
	return result
}

func fallbackContextFiles(candidates []string) []string {
	if len(candidates) > maxContextFiles {
		candidates = candidates[:maxContextFiles]
	}
	// Copy the slice so callers cannot observe later mutations to the input.
	return append([]string(nil), candidates...)
}

// contextFileWeight returns the weighted relevance score for a single candidate.
func contextFileWeight(path, kw string, historySet, recentSet map[string]bool) int {
	score := 0
	if kw != "" && strings.Contains(strings.ToLower(path), kw) {
		score += 5
	}
	if historySet[path] {
		score += 3
	}
	if recentSet[path] {
		score += 1
	}
	return score
}

// loadEditHistorySet returns the set of file paths that appear in successful
// patch history recorded on the memories branch. The underlying metric
// (ProblemFileCounts) accumulates file names from every successfully applied
// patch across runs; despite the field name it represents successful-edit
// frequency, not error frequency.
func loadEditHistorySet() map[string]bool {
	m, err := memory.LoadMetricsFromBranch()
	if err != nil {
		logInfo("edit_history_load_failed", "", err.Error())
		return nil
	}
	if len(m.ProblemFileCounts) == 0 {
		return nil
	}
	set := make(map[string]bool, len(m.ProblemFileCounts))
	for path := range m.ProblemFileCounts {
		set[path] = true
	}
	return set
}

// loadRecentFilesSet returns the set of Go files modified in the last 7 days
// according to git log.
func loadRecentFilesSet() map[string]bool {
	if cachedRecentFilesSetLoaded {
		return cachedRecentFilesSet
	}
	out, err := gitRecentFilesOutput()
	if err != nil {
		return nil
	}
	set := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasSuffix(line, ".go") {
			set[line] = true
		}
	}
	cachedRecentFilesSet = set
	cachedRecentFilesSetLoaded = true
	return cachedRecentFilesSet
}
