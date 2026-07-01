package main

import (
	"fmt"
	"sort"
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
// Any parse errors are intentionally ignored: AnalyzeFiles returns a partial
// SymbolMap for the files that did parse, which is sufficient for planning.
func symbolTasksForFiles(parentID string, files []string) []Task {
	sm, _ := audit.AnalyzeFiles(files)
	if len(sm.Functions) == 0 && len(sm.Structs) == 0 {
		return nil
	}
	return tasksFromSymbolMap(parentID, sm)
}

func tasksFromSymbolMap(parentID string, sm *audit.SymbolMap) []Task {
	var out []Task
	idx := 1

	funcKeys := make([]string, 0, len(sm.Functions))
	for k := range sm.Functions {
		funcKeys = append(funcKeys, k)
	}
	sort.Strings(funcKeys)

	for _, key := range funcKeys {
		fbs := sm.Functions[key]
		sort.Slice(fbs, func(i, j int) bool {
			if fbs[i].File != fbs[j].File {
				return fbs[i].File < fbs[j].File
			}
			return fbs[i].StartLine < fbs[j].StartLine
		})
		for _, fb := range fbs {
			st := SymbolTask{Change: funcChangeType(fb), Symbol: fb.Name, File: fb.File}
			out = append(out, generateSymbolTask(fmt.Sprintf("%s.s%d", parentID, idx), st))
			idx++
		}
	}

	structKeys := make([]string, 0, len(sm.Structs))
	for k := range sm.Structs {
		structKeys = append(structKeys, k)
	}
	sort.Strings(structKeys)

	for _, key := range structKeys {
		sds := sm.Structs[key]
		sort.Slice(sds, func(i, j int) bool {
			if sds[i].File != sds[j].File {
				return sds[i].File < sds[j].File
			}
			return sds[i].StartLine < sds[j].StartLine
		})
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
