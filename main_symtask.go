package main

import (
	"fmt"
	"strings"

	"github.com/opd-ai/orchestrator/audit"
)

// SymbolChangeType describes the kind of symbol-level change a task performs.
type SymbolChangeType string

const (
	ChangeAddFunc       SymbolChangeType = "add_function"
	ChangeModStruct     SymbolChangeType = "modify_struct"
	ChangeUpdateMethod  SymbolChangeType = "update_method"
	ChangeAdjustImports SymbolChangeType = "adjust_imports"
)

// SymbolTask describes a code change scoped to a single named symbol.
type SymbolTask struct {
	Change SymbolChangeType
	Symbol string
	File   string
}

// generateSymbolTask creates a Task scoped to a single symbol.
func generateSymbolTask(id string, st SymbolTask) Task {
	return Task{
		ID:          id,
		Description: symbolDescription(st),
		Files:       []string{st.File},
		Status:      "pending",
	}
}

// symbolTasksForFiles analyzes the given files and returns one Task per symbol.
// It is used by the deterministic planning engine to decompose file-level
// tasks into atomic, symbol-scoped units.
func symbolTasksForFiles(parentID string, files []string) []Task {
	sm, err := audit.AnalyzeFiles(files)
	if err != nil || (len(sm.Functions) == 0 && len(sm.Structs) == 0) {
		return nil
	}
	return tasksFromSymbolMap(parentID, sm)
}

func tasksFromSymbolMap(parentID string, sm *audit.SymbolMap) []Task {
	var out []Task
	idx := 1
	for _, fbs := range sm.Functions {
		for _, fb := range fbs {
			st := SymbolTask{Change: funcChangeType(fb), Symbol: fb.Name, File: fb.File}
			out = append(out, generateSymbolTask(fmt.Sprintf("%s.s%d", parentID, idx), st))
			idx++
		}
	}
	for _, sds := range sm.Structs {
		for _, sd := range sds {
			st := SymbolTask{Change: ChangeModStruct, Symbol: sd.Name, File: sd.File}
			out = append(out, generateSymbolTask(fmt.Sprintf("%s.s%d", parentID, idx), st))
			idx++
		}
	}
	return out
}

func funcChangeType(fb audit.FuncBoundary) SymbolChangeType {
	if fb.Receiver != "" {
		return ChangeUpdateMethod
	}
	return ChangeAddFunc
}

func symbolDescription(st SymbolTask) string {
	switch st.Change {
	case ChangeAddFunc:
		return fmt.Sprintf("Add function %s to %s", st.Symbol, st.File)
	case ChangeModStruct:
		return fmt.Sprintf("Modify struct %s in %s", st.Symbol, st.File)
	case ChangeUpdateMethod:
		return fmt.Sprintf("Update method %s in %s", st.Symbol, st.File)
	case ChangeAdjustImports:
		return fmt.Sprintf("Adjust imports in %s", st.File)
	default:
		return fmt.Sprintf("Change %s in %s", st.Symbol, st.File)
	}
}

// relevantToTask reports whether the symbol name appears in the task description.
func relevantToTask(description, symbolName string) bool {
	return strings.Contains(strings.ToLower(description), strings.ToLower(symbolName))
}
