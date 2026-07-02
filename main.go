package main

/*
Advanced Autonomous Engineering Orchestrator

Features:
- Document-driven task generation (AUDIT → ROADMAP)
- Cross-document deduplication
- DAG-based execution (depends_on)
- Automatic task splitting on repeated failure
- Smart repo context detection
- Git branch isolation
- Patch validation
- Structured JSON logging
- Self-hosted OpenAI-compatible endpoint

Assumptions:
- Self-hosted endpoint compatible with OpenAI chat API
- `patch`, `git`, `go` available
*/

import (
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/opd-ai/orchestrator/audit"
)

var goFileMentionRe = regexp.MustCompile(`\b[\w./-]+\.go\b`)

// inferenceLatencyTotal and inferenceCallCount accumulate per-call latency
// across a run and are reset at the start of each execution mode.
// Both are updated atomically because callLLMWithModel may be invoked from
// speculative goroutines concurrently.
var (
	inferenceLatencyTotal int64
	inferenceCallCount    int64
)

// maxBytesPerFile caps the number of bytes contributed by a single file to the
// prompt context. Files exceeding this limit are replaced with their signature-only
// summary via extractSignatures so the model still sees the callable surface.
const maxBytesPerFile = 2000

// ensureTasksFile bootstraps tasks from planning documents and audit findings when tasks.json is absent.
func ensureTasksFile() {
	if _, err := os.Stat(tasksFile); err == nil {
		return
	}

	docOrder := []struct {
		Name   string
		Prefix string
	}{
		{"GAPS.md", "G"},
		{"GOALS.md", "O"},
		{"PLAN.md", "P"},
		{"ROADMAP.md", "R"},
	}

	var allTasks []Task
	seen := map[string]bool{}
	docsFound := false
	taskGenerationFailures := 0

	for _, doc := range docOrder {
		if _, err := os.Stat(doc.Name); err != nil {
			continue
		}
		docsFound = true

		data, err := os.ReadFile(doc.Name)
		if err != nil {
			logError("doc_read_failed", "", err.Error())
			taskGenerationFailures++
			continue
		}
		generated, err := generateTasksFromDoc(doc.Name, string(data))
		if err != nil {
			logError("planner_failed", "", fmt.Sprintf("doc=%s err=%s", doc.Name, err.Error()))
			taskGenerationFailures++
			continue
		}

		for i, t := range generated {
			hash := hashString(t.Description)
			if seen[hash] {
				continue
			}
			seen[hash] = true

			t.ID = fmt.Sprintf("%s%d", doc.Prefix, i+1)
			t.Status = "pending"
			t.Hash = hash
			if len(t.Files) == 0 {
				t.Files = extractMentionedGoFiles(t.Description)
			}
			allTasks = append(allTasks, t)
		}
	}
	allTasks = mergeAuditFindings(allTasks, seen)

	if len(allTasks) == 0 {
		if docsFound && taskGenerationFailures > 0 {
			logFatal("task_generation_failed", "Planning documents found but task generation failed")
		}
		logFatal("no_documents_found", "No planning documents found")
	}

	allTasks = injectFileOverlapDeps(allTasks)
	tf := TaskFile{Tasks: allTasks}
	saveTasks(tf)
	logInfo("tasks_bootstrap_complete", "", fmt.Sprintf("%d tasks", len(allTasks)))
}

////////////////////////////////////////////////////////////
// DAG EXECUTION
////////////////////////////////////////////////////////////

func nextExecutableTask(tf *TaskFile) *Task {
	var selected *Task
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status != "pending" || !depsSatisfied(tf, t) {
			continue
		}
		if selected == nil || taskPriority(t) < taskPriority(selected) {
			selected = t
		}
	}
	if selected == nil {
		return nil
	}
	selected.Status = "in_progress"
	return selected
}

func mergeAuditFindings(allTasks []Task, seen map[string]bool) []Task {
	if _, err := os.Stat(auditOutput); err != nil {
		return allTasks
	}
	findings, err := audit.LoadFindings(auditOutput)
	if err != nil {
		logError("audit_findings_load_failed", "", err.Error())
		return allTasks
	}

	nextID := nextTaskIDIndex(allTasks, "A")
	for _, finding := range findings {
		severity := strings.ToUpper(strings.TrimSpace(finding.Severity))
		if severity != "HIGH" && severity != "CRITICAL" {
			continue
		}
		desc := fmt.Sprintf("[AUDIT-%s] %s", severity, strings.TrimSpace(finding.Description))
		hash := hashString(desc)
		if seen[hash] {
			continue
		}
		seen[hash] = true
		allTasks = append(allTasks, Task{
			ID:          fmt.Sprintf("A%d", nextID),
			Description: desc,
			Status:      "pending",
			Hash:        hash,
		})
		nextID++
	}
	return allTasks
}

func nextTaskIDIndex(tasks []Task, prefix string) int {
	maxID := 0
	for _, task := range tasks {
		if !strings.HasPrefix(task.ID, prefix) {
			continue
		}
		var current int
		if _, err := fmt.Sscanf(task.ID[len(prefix):], "%d", &current); err == nil && current > maxID {
			maxID = current
		}
	}
	return maxID + 1
}

func taskPriority(task *Task) int {
	upperDesc := strings.ToUpper(task.Description)
	if strings.HasPrefix(upperDesc, "[AUDIT-HIGH]") ||
		strings.HasPrefix(upperDesc, "[AUDIT-CRITICAL]") {
		return 0
	}
	return 1
}

func extractMentionedGoFiles(text string) []string {
	indices := goFileMentionRe.FindAllStringIndex(text, -1)
	if len(indices) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(indices))
	files := make([]string, 0, len(indices))
	for _, idx := range indices {
		start, end := idx[0], idx[1]
		if (start > 0 && (text[start-1] == '.' || text[start-1] == '/')) ||
			(end < len(text) && (text[end] == '.' || text[end] == '/')) {
			continue
		}
		match := text[start:end]
		clean := filepath.Clean(match)
		if !isSafeMentionedGoFile(clean) {
			continue
		}
		if seen[clean] {
			continue
		}
		seen[clean] = true
		files = append(files, clean)
	}
	sort.Strings(files)
	return files
}

func isSafeMentionedGoFile(path string) bool {
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(path)
	return clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func depsSatisfied(tf *TaskFile, t *Task) bool {
	for _, dep := range t.DependsOn {
		if !isComplete(tf, dep) {
			return false
		}
	}
	return true
}

func isComplete(tf *TaskFile, id string) bool {
	for _, t := range tf.Tasks {
		if t.ID == id {
			return t.Status == "complete"
		}
	}
	return false
}

////////////////////////////////////////////////////////////
// SMART CONTEXT
////////////////////////////////////////////////////////////

func resolveContextFiles(task *Task) []string {
	if len(task.Files) > 0 {
		return task.Files
	}

	out, _ := exec.Command("git", "ls-files").Output()
	files := strings.Split(string(out), "\n")

	var matched []string
	for _, f := range files {
		if strings.HasSuffix(f, ".go") &&
			strings.Contains(strings.ToLower(f), keyword(task.Description)) {
			matched = append(matched, f)
		}
		if len(matched) >= maxContextFiles {
			break
		}
	}

	return matched
}

func keyword(desc string) string {
	re := regexp.MustCompile(`[a-zA-Z]+`)
	words := re.FindAllString(desc, -1)
	if len(words) == 0 {
		return ""
	}
	return strings.ToLower(words[0])
}

func gatherFileContext(files []string) string {
	var b strings.Builder
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		b.WriteString("FILE: " + f + "\n")
		if len(data) > maxBytesPerFile {
			b.Write(extractSignatures(data))
		} else {
			b.Write(data)
		}
		b.WriteString("\n\n")
	}
	return b.String()
}

// extractSignatures returns only the declaration lines from data: lines beginning
// with "func ", "type ", "var ", or "const ". Used as a fallback when a file
// exceeds maxBytesPerFile so the model still sees the callable surface. Output is
// capped at maxBytesPerFile bytes to enforce the per-file prompt budget.
func extractSignatures(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	out := make([][]byte, 0, len(lines))
	total := 0
	for _, line := range lines {
		t := bytes.TrimLeft(line, " \t")
		if bytes.HasPrefix(t, []byte("func ")) ||
			bytes.HasPrefix(t, []byte("type ")) ||
			bytes.HasPrefix(t, []byte("var ")) ||
			bytes.HasPrefix(t, []byte("const ")) {
			if total+len(line)+1 > maxBytesPerFile {
				break
			}
			out = append(out, line)
			total += len(line) + 1
		}
	}
	return bytes.Join(out, []byte("\n"))
}

////////////////////////////////////////////////////////////
// PATCH + BUILD
////////////////////////////////////////////////////////////

func applyPatch(diff string) error {
	cmd := exec.Command("patch", "-p1")
	cmd.Stdin = strings.NewReader(diff)
	return cmd.Run()
}

func revertPatch(diff string) error {
	cmd := exec.Command("patch", "-p1", "-R")
	cmd.Stdin = strings.NewReader(diff)
	return cmd.Run()
}

func build() string {
	cmd := exec.Command("sh", "-c", "go build ./... && go test ./...")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out)
	}
	return ""
}

func markBlocked(task *Task) {
	task.Status = "blocked"
}

////////////////////////////////////////////////////////////
// UTIL
////////////////////////////////////////////////////////////

func generateTasksFromDoc(docType, content string) ([]Task, error) {
	prompt := fmt.Sprintf(`
Decompose into atomic tasks.
Return JSON array only.

Document:
%s

Content:
%s
`, docType, content)

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp := callLLMWithModel(promptWithMemory(prompt), 0.6, roleModel(plannerModelName))

		clean, err := extractJSON(resp)
		if err != nil {
			logError("planner_invalid_json", "", fmt.Sprintf("attempt=%d err=%s", attempt+1, err.Error()))
			continue
		}

		var tasks []Task
		if err := json.Unmarshal([]byte(clean), &tasks); err != nil {
			logError("planner_invalid_json", "", fmt.Sprintf("attempt=%d err=%s", attempt+1, err.Error()))
			continue
		}
		return tasks, nil
	}
	return nil, fmt.Errorf("planner failed to return valid JSON after %d attempts", maxRetries)
}

// callLLMWithTemp calls the LLM endpoint with the executor model and given temperature.
func callLLMWithTemp(prompt string, temperature float64) string {
	return callLLMWithModel(prompt, temperature, activeExecutorModel())
}

// callLLMWithModel calls the LLM endpoint with explicit model and temperature
// and logs prompt/completion token usage. Prompt truncation is handled once
// upstream by compressPrompt (via promptWithMemory); the redundant
// enforceTokenBudget call has been removed to unify truncation strategy (F-17).
func callLLMWithModel(prompt string, temperature float64, model string) string {
	body := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": temperature,
	}
	b, _ := json.Marshal(body)
	start := time.Now()
	resp, err := http.Post(llmEndpoint, "application/json", bytes.NewBuffer(b))
	if err != nil {
		logFatal("llm_call_failed", err.Error())
	}
	latencyMs := time.Since(start).Milliseconds()
	atomic.AddInt64(&inferenceLatencyTotal, latencyMs)
	atomic.AddInt64(&inferenceCallCount, 1)
	defer resp.Body.Close()

	out, _ := io.ReadAll(resp.Body)

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(out, &parsed); err != nil {
		logError("llm_parse_failed", "", err.Error())
	}
	logInfo("token_usage", "", fmt.Sprintf(
		"model=%s prompt=%d completion=%d total=%d latency_ms=%d",
		model, parsed.Usage.PromptTokens, parsed.Usage.CompletionTokens, parsed.Usage.TotalTokens, latencyMs,
	))
	if len(parsed.Choices) == 0 {
		logFatal("llm_empty_response", "no choices in LLM response")
	}
	return parsed.Choices[0].Message.Content
}

func loadTasks() TaskFile {
	data, _ := os.ReadFile(tasksFile)
	var tf TaskFile
	if err := json.Unmarshal(data, &tf); err != nil {
		logFatal("tasks_corrupt", err.Error())
	}
	return tf
}

func saveTasks(tf TaskFile) {
	b, _ := json.MarshalIndent(tf, "", "  ")
	os.WriteFile(tasksFile, b, 0o644)
}

func gitCommit(task *Task) error {
	if err := exec.Command("git", "add", ".").Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if err := exec.Command("git", "commit", "-m", "Task "+task.ID+": "+task.Description).Run(); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

func filesTouched(diff string) []string {
	lines := strings.Split(diff, "\n")
	set := map[string]bool{}
	for _, l := range lines {
		if strings.HasPrefix(l, "+++ b/") {
			set[strings.TrimPrefix(l, "+++ b/")] = true
		}
	}
	var out []string
	for k := range set {
		out = append(out, k)
	}
	return out
}

func lineCount(s string) int {
	return len(strings.Split(s, "\n"))
}

func hashString(s string) string {
	h := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(s))))
	return fmt.Sprintf("%x", h)
}

////////////////////////////////////////////////////////////
// LOGGING
////////////////////////////////////////////////////////////

func logFatal(event, msg string) {
	log("FATAL", event, "", msg)
	os.Exit(1)
}
