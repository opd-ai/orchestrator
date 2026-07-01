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
	"regexp"
	"strings"
)

func ensureTasksFile() {
	if _, err := os.Stat(tasksFile); err == nil {
		return
	}

	docOrder := []struct {
		Name   string
		Prefix string
	}{
		{"AUDIT.md", "A"},
		{"GAPS.md", "G"},
		{"GOALS.md", "O"},
		{"PLAN.md", "P"},
		{"ROADMAP.md", "R"},
	}

	var allTasks []Task
	seen := map[string]bool{}

	for _, doc := range docOrder {
		if _, err := os.Stat(doc.Name); err != nil {
			continue
		}

		data, _ := os.ReadFile(doc.Name)
		generated := generateTasksFromDoc(doc.Name, string(data))

		for i, t := range generated {
			hash := hashString(t.Description)
			if seen[hash] {
				continue
			}
			seen[hash] = true

			t.ID = fmt.Sprintf("%s%d", doc.Prefix, i+1)
			t.Status = "pending"
			t.Hash = hash
			allTasks = append(allTasks, t)
		}
	}

	if len(allTasks) == 0 {
		logFatal("no_documents_found", "No planning documents found")
	}

	tf := TaskFile{Tasks: allTasks}
	saveTasks(tf)
	logInfo("tasks_bootstrap_complete", "", fmt.Sprintf("%d tasks", len(allTasks)))
}

////////////////////////////////////////////////////////////
// DAG EXECUTION
////////////////////////////////////////////////////////////

func nextExecutableTask(tf *TaskFile) *Task {
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status == "pending" && depsSatisfied(tf, t) {
			t.Status = "in_progress"
			return t
		}
	}
	return nil
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
		b.Write(data)
		b.WriteString("\n\n")
	}
	return b.String()
}

////////////////////////////////////////////////////////////
// PATCH + BUILD
////////////////////////////////////////////////////////////

func applyPatch(diff string) error {
	cmd := exec.Command("patch", "-p1")
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

func generateTasksFromDoc(docType, content string) []Task {
	prompt := fmt.Sprintf(`
Decompose into atomic tasks.
Return JSON array only.

Document:
%s

Content:
%s
`, docType, content)

	resp := callLLM(promptWithMemory(prompt))

	clean, err := extractJSON(resp)
	if err != nil {
		logFatal("planner_invalid_json", err.Error())
	}

	var tasks []Task
	if err := json.Unmarshal([]byte(clean), &tasks); err != nil {
		logFatal("planner_invalid_json", err.Error())
	}
	return tasks
}

func callLLM(prompt string) string {
	body := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.6,
	}
	b, _ := json.Marshal(body)
	resp, err := http.Post(llmEndpoint, "application/json", bytes.NewBuffer(b))
	if err != nil {
		logFatal("llm_call_failed", err.Error())
	}
	defer resp.Body.Close()

	out, _ := io.ReadAll(resp.Body)

	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	json.Unmarshal(out, &parsed)
	return parsed.Choices[0].Message.Content
}

func loadTasks() TaskFile {
	data, _ := os.ReadFile(tasksFile)
	var tf TaskFile
	json.Unmarshal(data, &tf)
	return tf
}

func saveTasks(tf TaskFile) {
	b, _ := json.MarshalIndent(tf, "", "  ")
	os.WriteFile(tasksFile, b, 0o644)
}

func gitCommit(task *Task) {
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "Task "+task.ID+": "+task.Description).Run()
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
