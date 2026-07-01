package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
)

func DetectHotspots(files []string) []Hotspot {
	var hotspots []Hotspot

	for _, file := range files {
		loc := countLines(file)
		if loc < 300 {
			continue
		}

		complexity := estimateComplexity(file)

		hotspots = append(hotspots, Hotspot{
			File:       file,
			LOC:        loc,
			Complexity: complexity,
		})
	}

	return hotspots
}

func estimateComplexity(file string) int {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return 0
	}

	score := 0
	ast.Inspect(node, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.SwitchStmt, *ast.TypeSwitchStmt:
			score++
		}
		return true
	})

	return score
}

// DeadFunctionScan returns unexported function names in the given files
// that appear to have no call sites within the same file set.
// It excludes init and main. Results are sorted for determinism.
func DeadFunctionScan(files []string) []string {
	fset := token.NewFileSet()
	defined := make(map[string]bool)
	called := make(map[string]bool)

	for _, file := range files {
		node, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			continue
		}
		ast.Inspect(node, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.FuncDecl:
				if !v.Name.IsExported() && v.Name.Name != "init" && v.Name.Name != "main" {
					defined[v.Name.Name] = true
				}
			case *ast.CallExpr:
				if ident, ok := v.Fun.(*ast.Ident); ok {
					called[ident.Name] = true
				}
				if sel, ok := v.Fun.(*ast.SelectorExpr); ok {
					called[sel.Sel.Name] = true
				}
			}
			return true
		})
	}

	var dead []string
	for name := range defined {
		if !called[name] {
			dead = append(dead, name)
		}
	}
	slices.Sort(dead)
	return dead
}
