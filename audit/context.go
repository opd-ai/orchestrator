package audit

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
)

func BuildAuditContext(cluster Cluster, graph *DependencyGraph) AuditContext {
	var exports []SymbolInfo
	importSet := make(map[string]bool)
	callDensity := make(map[string]int)
	fileSet := make(map[string]bool)

	for _, pkgPath := range cluster.Packages {
		pkg, ok := graph.Packages[pkgPath]
		if !ok {
			continue
		}

		for _, imp := range pkg.Imports {
			importSet[imp] = true
		}

		callDensity[pkgPath] = inboundImportCount(graph, pkgPath)
		exports = append(exports, collectSymbolInfos(pkgPath, pkg.Files)...)

		for _, file := range pkg.Files {
			if filepath.Ext(file) == ".go" {
				fileSet[file] = true
			}
		}
	}

	var imports []string
	for imp := range importSet {
		imports = append(imports, imp)
	}
	slices.Sort(imports)

	var files []string
	for file := range fileSet {
		files = append(files, file)
	}
	slices.Sort(files)

	summary := fmt.Sprintf(
		"Cluster %s: %d packages, %d LOC",
		cluster.ID,
		len(cluster.Packages),
		cluster.TotalLOC,
	)

	return AuditContext{
		ClusterSummary: summary,
		Exports:        exports,
		Imports:        imports,
		Hotspots:       DetectHotspots(files),
		CallDensity:    callDensity,
	}
}

func FormatContextForLLM(ctx AuditContext) string {
	var b strings.Builder

	b.WriteString("AUDIT CONTEXT\n")
	b.WriteString(ctx.ClusterSummary + "\n")

	b.WriteString("\nIMPORTS:\n")
	for _, imp := range ctx.Imports {
		b.WriteString("- " + imp + "\n")
	}

	b.WriteString("\nCALL DENSITY:\n")
	for pkg, d := range ctx.CallDensity {
		b.WriteString(fmt.Sprintf("- %s: %d\n", pkg, d))
	}

	return b.String()
}

func inboundImportCount(graph *DependencyGraph, target string) int {
	count := 0
	for _, imports := range graph.Edges {
		if slices.Contains(imports, target) {
			count++
		}
	}
	return count
}

func collectSymbolInfos(pkgPath string, files []string) []SymbolInfo {
	fset := token.NewFileSet()
	var symbols []SymbolInfo

	for _, file := range files {
		node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			continue
		}
		for _, decl := range node.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if !d.Name.IsExported() {
					continue
				}
				symbol := SymbolInfo{
					Name:     d.Name.Name,
					Kind:     "func",
					Exported: true,
					Package:  pkgPath,
				}
				if d.Recv != nil && len(d.Recv.List) > 0 {
					symbol.Kind = "method"
					symbol.Receiver = exprString(d.Recv.List[0].Type)
				}
				symbols = append(symbols, symbol)
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if !s.Name.IsExported() {
							continue
						}
						kind := "type"
						if _, ok := s.Type.(*ast.InterfaceType); ok {
							kind = "interface"
						}
						symbols = append(symbols, SymbolInfo{
							Name:     s.Name.Name,
							Kind:     kind,
							Exported: true,
							Package:  pkgPath,
						})
					case *ast.ValueSpec:
						kind := strings.ToLower(d.Tok.String())
						for _, name := range s.Names {
							if !name.IsExported() {
								continue
							}
							symbols = append(symbols, SymbolInfo{
								Name:     name.Name,
								Kind:     kind,
								Exported: true,
								Package:  pkgPath,
							})
						}
					}
				}
			}
		}
	}

	return symbols
}

func exprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprString(e.X)
	case *ast.SelectorExpr:
		return exprString(e.X) + "." + e.Sel.Name
	default:
		return ""
	}
}
