package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
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
