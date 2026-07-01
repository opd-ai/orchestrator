package main

import (
	"testing"

	"github.com/opd-ai/orchestrator/audit"
)

func TestSymbolDescription(t *testing.T) {
	tests := []struct {
		task SymbolTask
		want string
	}{
		{SymbolTask{Change: ChangeAddFunc, Symbol: "NewFoo", File: "foo.go"}, "Add function NewFoo to foo.go"},
		{SymbolTask{Change: ChangeModStruct, Symbol: "Config", File: "config.go"}, "Modify struct Config in config.go"},
		{SymbolTask{Change: ChangeUpdateMethod, Symbol: "Validate", File: "types.go"}, "Update method Validate in types.go"},
		{SymbolTask{Change: ChangeAdjustImports, Symbol: "", File: "main.go"}, "Adjust imports in main.go"},
		{SymbolTask{Change: "unknown", Symbol: "X", File: "x.go"}, "Change X in x.go"},
	}
	for _, tc := range tests {
		got := symbolDescription(tc.task)
		if got != tc.want {
			t.Errorf("symbolDescription(%v) = %q, want %q", tc.task, got, tc.want)
		}
	}
}

func TestGenerateSymbolTask(t *testing.T) {
	st := SymbolTask{Change: ChangeAddFunc, Symbol: "NewFoo", File: "foo.go"}
	task := generateSymbolTask("T1", st)

	if task.ID != "T1" {
		t.Errorf("expected ID T1, got %s", task.ID)
	}
	if len(task.Files) != 1 || task.Files[0] != "foo.go" {
		t.Errorf("expected Files=[foo.go], got %v", task.Files)
	}
	if task.Status != "pending" {
		t.Errorf("expected pending status, got %s", task.Status)
	}
	if task.Description != "Add function NewFoo to foo.go" {
		t.Errorf("unexpected description: %q", task.Description)
	}
}

func TestFuncChangeType(t *testing.T) {
	plain := audit.FuncBoundary{Name: "NewFoo"}
	if funcChangeType(plain) != ChangeAddFunc {
		t.Error("expected ChangeAddFunc for function without receiver")
	}

	method := audit.FuncBoundary{Name: "Validate", Receiver: "*Config"}
	if funcChangeType(method) != ChangeUpdateMethod {
		t.Error("expected ChangeUpdateMethod for method with receiver")
	}
}

func TestRelevantToTask(t *testing.T) {
	if !relevantToTask("Update validatePatch in main_validatepatch.go", "validatePatch") {
		t.Error("validatePatch should be relevant to the description")
	}
	if relevantToTask("Update something else", "validatePatch") {
		t.Error("validatePatch should not be relevant when not in description")
	}
}
