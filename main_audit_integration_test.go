package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/opd-ai/orchestrator/audit"
)

func TestMergeAuditFindingsInjectsHighAndCritical(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	prevAuditOutput := auditOutput
	t.Cleanup(func() { auditOutput = prevAuditOutput })
	auditOutput = filepath.Join(tmpDir, "audit_findings.json")
	findings := []audit.Finding{
		{Severity: "low", Description: "ignore me"},
		{Severity: "HIGH", Description: "fix high issue"},
		{Severity: "critical", Description: "fix critical issue"},
	}
	data, err := json.Marshal(findings)
	if err != nil {
		t.Fatalf("marshal findings: %v", err)
	}
	if err := os.WriteFile(auditOutput, data, 0o644); err != nil {
		t.Fatalf("write findings file: %v", err)
	}

	existing := []Task{{ID: "R1", Description: "normal", Status: "pending"}}
	seen := map[string]bool{hashString("normal"): true}
	merged := mergeAuditFindings(existing, seen)

	if len(merged) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(merged))
	}
	if merged[1].ID != "A1" || merged[2].ID != "A2" {
		t.Fatalf("unexpected injected IDs: %+v", merged)
	}
	if taskPriority(&merged[1]) != 0 || taskPriority(&merged[2]) != 0 {
		t.Fatalf("expected audit tasks to be highest priority: %+v", merged)
	}
}

func TestNextExecutableTaskPrioritizesAuditFindings(t *testing.T) {
	tf := TaskFile{Tasks: []Task{
		{ID: "R1", Description: "normal task", Status: "pending"},
		{ID: "A2", Description: "[AUDIT-HIGH] urgent", Status: "pending"},
	}}

	task := nextExecutableTask(&tf)
	if task == nil {
		t.Fatal("expected task")
	}
	if task.ID != "A2" {
		t.Fatalf("expected audit task first, got %s", task.ID)
	}
}

func TestExtractMentionedGoFiles(t *testing.T) {
	got := extractMentionedGoFiles("Update main_exec.go and audit/context.go and main_exec.go")
	if len(got) != 2 {
		t.Fatalf("expected 2 unique files, got %v", got)
	}
	if got[0] != "audit/context.go" || got[1] != "main_exec.go" {
		t.Fatalf("unexpected files: %v", got)
	}
}
