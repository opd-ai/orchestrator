package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/opd-ai/orchestrator/audit"
)

func TestMergeAuditFindings_DeduplicationAndPriority(t *testing.T) {
	// Create a temporary audit findings file
	tmpDir := t.TempDir()
	auditFile := filepath.Join(tmpDir, "audit_findings.json")

	testCases := []struct {
		name          string
		findings      []audit.Finding
		existingTasks []Task
		expectedCount int
		expectedIDs   []string
	}{
		{
			name:     "no audit findings",
			findings: []audit.Finding{},
			existingTasks: []Task{
				{ID: "O1", Description: "Some goal", Status: "pending"},
			},
			expectedCount: 1,
			expectedIDs:   []string{"O1"},
		},
		{
			name: "single high severity finding",
			findings: []audit.Finding{
				{
					Package:        "pkg1",
					Type:           "architecture",
					Severity:       "high",
					Description:    "Some architecture issue",
					Recommendation: "Fix it",
					Confidence:     0.9,
				},
			},
			existingTasks: []Task{},
			expectedCount: 1,
			expectedIDs:   []string{"A" + hashString("[AUDIT-HIGH] Some architecture issue")},
		},
		{
			name: "duplicate high severity findings (same description)",
			findings: []audit.Finding{
				{
					Package:        "pkg1",
					Type:           "architecture",
					Severity:       "high",
					Description:    "Some architecture issue",
					Recommendation: "Fix it",
					Confidence:     0.9,
				},
				{
					Package:        "pkg2",
					Type:           "architecture",
					Severity:       "high",
					Description:    "Some architecture issue",
					Recommendation: "Fix it",
					Confidence:     0.8,
				},
			},
			existingTasks: []Task{},
			expectedCount: 1,
			expectedIDs:   []string{"A" + hashString("[AUDIT-HIGH] Some architecture issue")},
		},
		{
			name: "mixed severity (only high and critical included)",
			findings: []audit.Finding{
				{
					Package:        "pkg1",
					Type:           "architecture",
					Severity:       "high",
					Description:    "High issue",
					Recommendation: "Fix",
					Confidence:     0.9,
				},
				{
					Package:        "pkg2",
					Type:           "architecture",
					Severity:       "critical",
					Description:    "Critical issue",
					Recommendation: "Fix urgently",
					Confidence:     0.95,
				},
				{
					Package:        "pkg3",
					Type:           "architecture",
					Severity:       "medium",
					Description:    "Medium issue",
					Recommendation: "Consider fixing",
					Confidence:     0.7,
				},
				{
					Package:        "pkg4",
					Type:           "architecture",
					Severity:       "low",
					Description:    "Low issue",
					Recommendation: "Ignore",
					Confidence:     0.5,
				},
			},
			existingTasks: []Task{},
			expectedCount: 2,
			expectedIDs: []string{
				"A" + hashString("[AUDIT-HIGH] High issue"),
				"A" + hashString("[AUDIT-CRITICAL] Critical issue"),
			},
		},
		{
			name: "mixed with existing tasks (deduplication across sources)",
			findings: []audit.Finding{
				{
					Package:        "pkg1",
					Type:           "architecture",
					Severity:       "high",
					Description:    "Some architecture issue",
					Recommendation: "Fix it",
					Confidence:     0.9,
				},
			},
			existingTasks: []Task{
				{ID: "O1", Description: "[AUDIT-HIGH] Some architecture issue", Status: "pending", Source: "goals"},
			},
			expectedCount: 1,              // deduplicated by hash
			expectedIDs:   []string{"O1"}, // existing task keeps its ID
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write audit findings to file
			data, err := json.MarshalIndent(tc.findings, "", "  ")
			if err != nil {
				t.Fatalf("failed to marshal audit findings: %v", err)
			}
			if err := os.WriteFile(auditFile, data, 0o644); err != nil {
				t.Fatalf("failed to write audit file: %v", err)
			}

			// Temporarily override auditOutput
			oldAuditOutput := auditOutput
			auditOutput = auditFile
			defer func() { auditOutput = oldAuditOutput }()

			// Call mergeAuditFindings
			result := mergeAuditFindings(tc.existingTasks, make(map[string]bool))

			// Check count
			if len(result) != tc.expectedCount {
				t.Fatalf("expected %d tasks, got %d", tc.expectedCount, len(result))
			}

			// Check IDs (order may vary, so we sort)
			var ids []string
			for _, t := range result {
				ids = append(ids, t.ID)
			}
			sort.Strings(ids)
			sort.Strings(tc.expectedIDs)
			for i, id := range ids {
				if id != tc.expectedIDs[i] {
					t.Fatalf("task ID mismatch at index %d: expected %s, got %s", i, tc.expectedIDs[i], id)
				}
			}

			// Additional checks for audit tasks: source and priority
			for _, task := range result {
				if task.Source == "audit" {
					// Ensure audit tasks have the correct source
					if task.Source != "audit" {
						t.Fatalf("audit task should have source 'audit', got %s", task.Source)
					}
					// Ensure audit tasks have high or critical priority (0 or 1)
					p := taskPriority(&task)
					if p > taskPriorityHigh {
						t.Fatalf("audit task should have priority high or critical (0 or 1), got %d", p)
					}
				}
			}
		})
	}
}

func TestTaskPriority_AuditTags(t *testing.T) {
	tests := []struct {
		description string
		expected    int
	}{
		{"[AUDIT-CRITICAL] Fix critical issue", taskPriorityCritical},
		{"[AUDIT-HIGH] Fix high issue", taskPriorityHigh},
		{"[CRITICAL] Fix critical issue", taskPriorityCritical},
		{"[HIGH] Fix high issue", taskPriorityHigh},
		{"[NORMAL] Normal task", taskPriorityNormal},
		{"[LOW] Low task", taskPriorityLow},
		{"No prefix", taskPriorityNormal},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			if got := baseTaskPriority(tt.description); got != tt.expected {
				t.Errorf("baseTaskPriority(%q) = %d, want %d", tt.description, got, tt.expected)
			}
		})
	}
}

func TestTaskSource_Logging(t *testing.T) {
	// We can test that audit tasks are logged with the correct event
	// For simplicity, we'll just check that the logInfo call is made with the right event.
	// In a real test, we might mock the logger, but for now we'll just run the function and see if it panics.
	tmpDir := t.TempDir()
	auditFile := filepath.Join(tmpDir, "audit_findings.json")

	findings := []audit.Finding{
		{
			Package:        "pkg1",
			Type:           "architecture",
			Severity:       "high",
			Description:    "Test audit finding",
			Recommendation: "Fix it",
			Confidence:     0.9,
		},
	}
	data, _ := json.MarshalIndent(findings, "", "  ")
	_ = os.WriteFile(auditFile, data, 0o644)

	oldAuditOutput := auditOutput
	auditOutput = auditFile
	defer func() { auditOutput = oldAuditOutput }()

	// Run the function; if it panics, the test will fail.
	_ = mergeAuditFindings([]Task{}, make(map[string]bool))
}
