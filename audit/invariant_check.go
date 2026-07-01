package audit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

// InvariantViolation describes a single architectural invariant breach.
type InvariantViolation struct {
	Invariant string
	File      string
	Detail    string
}

func (v InvariantViolation) String() string {
	return fmt.Sprintf("%s in %s: %s", v.Invariant, v.File, v.Detail)
}

// CheckFileInvariants validates one Go source file against the loaded registry.
// Returns an empty slice when no violations are found.
func CheckFileInvariants(path string, reg *InvariantRegistry) []InvariantViolation {
	if reg == nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	text := string(data)
	lines := strings.Count(text, "\n") + 1

	var violations []InvariantViolation
	violations = append(violations, checkFileLengthInvariant(path, lines, reg)...)
	violations = append(violations, checkFunctionInvariants(path, reg)...)
	return violations
}

func checkFileLengthInvariant(path string, lines int, reg *InvariantRegistry) []InvariantViolation {
	for _, inv := range reg.Invariants {
		if inv.Name == "max_file_length" && inv.MaxValue > 0 && lines > inv.MaxValue {
			return []InvariantViolation{{
				Invariant: inv.Name,
				File:      path,
				Detail:    fmt.Sprintf("%d lines (max %d)", lines, inv.MaxValue),
			}}
		}
	}
	return nil
}

func checkFunctionInvariants(path string, reg *InvariantRegistry) []InvariantViolation {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil
	}

	maxFnLen, maxCC := 0, 0
	for _, inv := range reg.Invariants {
		switch inv.Name {
		case "max_function_length":
			maxFnLen = inv.MaxValue
		case "max_cyclomatic_complexity":
			maxCC = inv.MaxValue
		}
	}

	var violations []InvariantViolation
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		start := fset.Position(fn.Body.Lbrace)
		end := fset.Position(fn.Body.Rbrace)
		length := end.Line - start.Line
		if maxFnLen > 0 && length > maxFnLen {
			violations = append(violations, InvariantViolation{
				Invariant: "max_function_length",
				File:      path,
				Detail:    fmt.Sprintf("func %s: %d lines (max %d)", fn.Name.Name, length, maxFnLen),
			})
		}
		if maxCC > 0 {
			if cc := functionCyclomaticComplexity(fn); cc > maxCC {
				violations = append(violations, InvariantViolation{
					Invariant: "max_cyclomatic_complexity",
					File:      path,
					Detail:    fmt.Sprintf("func %s: complexity %d (max %d)", fn.Name.Name, cc, maxCC),
				})
			}
		}
	}
	return violations
}

// functionCyclomaticComplexity approximates cyclomatic complexity via
// branch-point counting within a function body.
func functionCyclomaticComplexity(fn *ast.FuncDecl) int {
	count := 1
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.SwitchStmt, *ast.SelectStmt,
			*ast.CaseClause, *ast.CommClause:
			count++
		}
		return true
	})
	return count
}
