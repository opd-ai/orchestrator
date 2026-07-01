package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
)

// FuncBoundary describes the source location of a function declaration.
type FuncBoundary struct {
	Name      string
	Receiver  string
	File      string
	StartLine int
	EndLine   int
}

// StructDef describes the source location of a struct type declaration and its field names.
type StructDef struct {
	Name      string
	File      string
	Fields    []string
	StartLine int
	EndLine   int
}

// SymbolMap maps symbol names to their source-level declarations across one or more files.
type SymbolMap struct {
	Functions map[string][]FuncBoundary
	Structs   map[string][]StructDef
}

// AnalyzeFile parses a single Go source file and returns its SymbolMap.
func AnalyzeFile(path string) (*SymbolMap, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, err
	}
	sm := newSymbolMap()
	extractSymbols(fset, node, path, sm)
	return sm, nil
}

// AnalyzeFiles parses multiple Go source files and merges their SymbolMaps.
// Files that fail to parse are silently skipped.
func AnalyzeFiles(paths []string) (*SymbolMap, error) {
	merged := newSymbolMap()
	for _, path := range paths {
		sm, err := AnalyzeFile(path)
		if err != nil {
			continue
		}
		mergeInto(merged, sm)
	}
	return merged, nil
}

func newSymbolMap() *SymbolMap {
	return &SymbolMap{
		Functions: make(map[string][]FuncBoundary),
		Structs:   make(map[string][]StructDef),
	}
}

func extractSymbols(fset *token.FileSet, node *ast.File, path string, sm *SymbolMap) {
	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			fb := funcBoundary(fset, d, path)
			sm.Functions[fb.Name] = append(sm.Functions[fb.Name], fb)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if sd, ok2 := structDef(fset, ts, path); ok2 {
					sm.Structs[sd.Name] = append(sm.Structs[sd.Name], sd)
				}
			}
		}
	}
}

func funcBoundary(fset *token.FileSet, d *ast.FuncDecl, path string) FuncBoundary {
	fb := FuncBoundary{
		Name:      d.Name.Name,
		File:      path,
		StartLine: fset.Position(d.Pos()).Line,
		EndLine:   fset.Position(d.End()).Line,
	}
	if d.Recv != nil && len(d.Recv.List) > 0 {
		fb.Receiver = exprString(d.Recv.List[0].Type)
	}
	return fb
}

// structDef extracts a StructDef from a TypeSpec if the type is a struct.
// Returns the definition and true on success, or an empty StructDef and false otherwise.
func structDef(fset *token.FileSet, ts *ast.TypeSpec, path string) (StructDef, bool) {
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return StructDef{}, false
	}
	sd := StructDef{
		Name:      ts.Name.Name,
		File:      path,
		StartLine: fset.Position(ts.Pos()).Line,
		EndLine:   fset.Position(ts.End()).Line,
	}
	for _, field := range st.Fields.List {
		for _, name := range field.Names {
			sd.Fields = append(sd.Fields, name.Name)
		}
	}
	return sd, true
}

// mergeInto appends all functions and structs from src into dst.
func mergeInto(dst, src *SymbolMap) {
	for name, fbs := range src.Functions {
		dst.Functions[name] = append(dst.Functions[name], fbs...)
	}
	for name, sds := range src.Structs {
		dst.Structs[name] = append(dst.Structs[name], sds...)
	}
}
