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

func BuildAuditContext(cluster Cluster, graph *DependencyGraph, needsFuncDAG bool) AuditContext {
	exports, imports, callDensity, files := clusterInputs(cluster, graph)

	summary := fmt.Sprintf(
		"Cluster %s: %d packages, %d LOC",
		cluster.ID,
		len(cluster.Packages),
		cluster.TotalLOC,
	)

	var dag *FuncDAG
	if needsFuncDAG {
		dag, _ = BuildFuncDAG(files)
	}

	return AuditContext{
		ClusterSummary: summary,
		Exports:        exports,
		Imports:        imports,
		Files:          files,
		Hotspots:       DetectHotspots(files),
		CallDensity:    callDensity,
		DeadFunctions:  DeadFunctionScan(files),
		FuncDAG:        dag,
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

// collectSymbolInfos parses exported symbols from all files in a package.
func collectSymbolInfos(pkgPath string, files []string) []SymbolInfo {
	fset := token.NewFileSet()
	var symbols []SymbolInfo
	for _, file := range files {
		symbols = append(symbols, symbolsFromFile(pkgPath, file, fset)...)
	}
	return symbols
}

// symbolsFromFile parses one Go source file and returns its exported symbols.
func symbolsFromFile(pkgPath, file string, fset *token.FileSet) []SymbolInfo {
	node, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return nil
	}
	var symbols []SymbolInfo
	for _, decl := range node.Decls {
		symbols = append(symbols, symbolsFromDecl(pkgPath, decl)...)
	}
	return symbols
}

// symbolsFromDecl dispatches a top-level declaration to the appropriate symbol extractor.
func symbolsFromDecl(pkgPath string, decl ast.Decl) []SymbolInfo {
	switch d := decl.(type) {
	case *ast.FuncDecl:
		return funcDeclSymbol(pkgPath, d)
	case *ast.GenDecl:
		return genDeclSymbols(pkgPath, d)
	}
	return nil
}

// funcDeclSymbol returns the SymbolInfo for an exported function or method declaration.
func funcDeclSymbol(pkgPath string, d *ast.FuncDecl) []SymbolInfo {
	if !d.Name.IsExported() {
		return nil
	}
	symbol := SymbolInfo{
		Name:     d.Name.Name,
		Kind:     "func",
		Exported: true,
		Package:  pkgPath,
		HasDoc:   d.Doc != nil && len(d.Doc.List) > 0,
	}
	if d.Recv != nil && len(d.Recv.List) > 0 {
		symbol.Kind = "method"
		symbol.Receiver = exprString(d.Recv.List[0].Type)
	}
	return []SymbolInfo{symbol}
}

// genDeclSymbols returns exported symbols from a grouped declaration (type/var/const).
func genDeclSymbols(pkgPath string, d *ast.GenDecl) []SymbolInfo {
	gdHasDoc := d.Doc != nil && len(d.Doc.List) > 0
	var symbols []SymbolInfo
	for _, spec := range d.Specs {
		symbols = append(symbols, specSymbols(pkgPath, d.Tok, spec, gdHasDoc)...)
	}
	return symbols
}

// specSymbols extracts exported symbols from a single spec within a GenDecl.
func specSymbols(pkgPath string, tok token.Token, spec ast.Spec, gdHasDoc bool) []SymbolInfo {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		return typeSpecSymbol(pkgPath, s, gdHasDoc)
	case *ast.ValueSpec:
		return valueSpecSymbols(pkgPath, tok, s, gdHasDoc)
	}
	return nil
}

// typeSpecSymbol returns the SymbolInfo for an exported type or interface.
func typeSpecSymbol(pkgPath string, s *ast.TypeSpec, gdHasDoc bool) []SymbolInfo {
	if !s.Name.IsExported() {
		return nil
	}
	kind := "type"
	var methods []string
	if iface, ok := s.Type.(*ast.InterfaceType); ok {
		kind = "interface"
		methods = interfaceMethodNames(iface)
	}
	hasDoc := gdHasDoc || (s.Doc != nil && len(s.Doc.List) > 0) || (s.Comment != nil && len(s.Comment.List) > 0)
	return []SymbolInfo{{
		Name: s.Name.Name, Kind: kind, Exported: true,
		Package: pkgPath, HasDoc: hasDoc, Methods: methods,
	}}
}

// valueSpecSymbols returns SymbolInfos for each exported name in a var/const spec.
func valueSpecSymbols(pkgPath string, tok token.Token, s *ast.ValueSpec, gdHasDoc bool) []SymbolInfo {
	kind := strings.ToLower(tok.String())
	hasDoc := gdHasDoc || (s.Doc != nil && len(s.Doc.List) > 0) || (s.Comment != nil && len(s.Comment.List) > 0)
	var symbols []SymbolInfo
	for _, name := range s.Names {
		if !name.IsExported() {
			continue
		}
		symbols = append(symbols, SymbolInfo{
			Name: name.Name, Kind: kind, Exported: true,
			Package: pkgPath, HasDoc: hasDoc,
		})
	}
	return symbols
}

// interfaceMethodNames returns the method names declared in an interface type.
func interfaceMethodNames(iface *ast.InterfaceType) []string {
	var names []string
	if iface.Methods == nil {
		return names
	}
	for _, field := range iface.Methods.List {
		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}
	return names
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

func clusterInputs(cluster Cluster, graph *DependencyGraph) ([]SymbolInfo, []string, map[string]int, []string) {
	var exports []SymbolInfo
	importSet := make(map[string]bool)
	callDensity := make(map[string]int)
	fileSet := make(map[string]bool)

	for _, pkgPath := range cluster.Packages {
		pkg, ok := graph.Packages[pkgPath]
		if !ok {
			continue
		}

		addImports(importSet, pkg.Imports)
		callDensity[pkgPath] = inboundImportCount(graph, pkgPath)
		exports = append(exports, collectSymbolInfos(pkgPath, pkg.Files)...)
		addGoFiles(fileSet, pkg.Files)
	}

	return exports, sortedKeys(importSet), callDensity, sortedKeys(fileSet)
}

func addImports(importSet map[string]bool, imports []string) {
	for _, imp := range imports {
		importSet[imp] = true
	}
}

func addGoFiles(fileSet map[string]bool, files []string) {
	for _, file := range files {
		if filepath.Ext(file) == ".go" {
			fileSet[file] = true
		}
	}
}

func sortedKeys(values map[string]bool) []string {
	var keys []string
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
