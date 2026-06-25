package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

const (
	modelName     = "local-27b"
	llmEndpoint   = "http://localhost:11434/v1/chat/completions"
	maxPatchLines = 50
	maxRetries    = 5
)

type Task struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Files       []string `json:"files"`
	RetryCount  int      `json:"retry_count"`
}

type TaskFile struct {
	Goal        string   `json:"goal"`
	Constraints []string `json:"constraints"`
	Tasks       []Task   `json:"tasks"`
}

func main() {
	for {
		tf := loadTasks()

		if len(tf.Tasks) == 0 {
			fmt.Println("No tasks found. Running planner...")
			runPlanner(&tf)
			saveTasks(tf)
			continue
		}

		task := nextPending(&tf)
		if task == nil {
			fmt.Println("All tasks complete.")
			return
		}

		fmt.Println("Running task:", task.ID)

		diff, err := executeTask(tf, task)
		if err != nil {
			fmt.Println("Task execution error:", err)
			task.Status = "failed"
			saveTasks(tf)
			continue
		}

		if err := applyPatch(diff); err != nil {
			fmt.Println("Patch apply failed:", err)
			task.Status = "failed"
			saveTasks(tf)
			continue
		}

		buildOut := build()

		if buildOut == "" {
			fmt.Println("Task complete:", task.ID)
			gitCommit(task)
			task.Status = "complete"
			saveTasks(tf)
			continue
		}

		for i := 0; i < maxRetries; i++ {
			fmt.Println("Fix attempt", i+1)
			fixDiff := fixTask(tf, task, buildOut)

			if err := applyPatch(fixDiff); err != nil {
				break
			}

			buildOut = build()
			if buildOut == "" {
				gitCommit(task)
				task.Status = "complete"
				break
			}
		}

		if task.Status != "complete" {
			task.Status = "blocked"
		}

		saveTasks(tf)
	}
}

func runPlanner(tf *TaskFile) {
	prompt := fmt.Sprintf("Break this goal into atomic tasks.\nGoal: %s\n", tf.Goal)
	resp := callLLM(prompt)
	json.Unmarshal([]byte(resp), &tf.Tasks)
}

func executeTask(tf TaskFile, task *Task) (string, error) {
	prompt := fmt.Sprintf("Implement task: %s\nReturn unified diff only.", task.Description)
	return callLLM(prompt), nil
}

func fixTask(tf TaskFile, task *Task, errors string) string {
	prompt := fmt.Sprintf("Fix these errors:\n%s\nReturn unified diff only.", errors)
	return callLLM(prompt)
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
		panic(err)
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

func applyPatch(diff string) error {
	if lineCount(diff) > maxPatchLines {
		return fmt.Errorf("patch too large")
	}

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

func gitCommit(task *Task) {
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "Task "+task.ID+": "+task.Description).Run()
}

func lineCount(s string) int {
	return len(strings.Split(s, "\n"))
}

func loadTasks() TaskFile {
	data, _ := os.ReadFile("tasks.json")
	var tf TaskFile
	json.Unmarshal(data, &tf)
	return tf
}

func saveTasks(tf TaskFile) {
	b, _ := json.MarshalIndent(tf, "", "  ")
	os.WriteFile("tasks.json", b, 0644)
}

func nextPending(tf *TaskFile) *Task {
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == "" || tf.Tasks[i].Status == "pending" {
			tf.Tasks[i].Status = "in_progress"
			return &tf.Tasks[i]
		}
	}
	return nil
}
