package audit

import (
	"go/ast"
	"os"

	"golang.org/x/tools/go/packages"
)

func LoadPackages(pattern string) (map[string]*PackageInfo, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedImports,
	}

	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*PackageInfo)

	for _, pkg := range pkgs {
		info := &PackageInfo{
			Name:    pkg.Name,
			Path:    pkg.PkgPath,
			Imports: make([]string, 0),
			Exports: make([]string, 0),
			Files:   pkg.GoFiles,
		}

		for imp := range pkg.Imports {
			info.Imports = append(info.Imports, imp)
		}

		info.Exports = extractExportedNames(pkg.Syntax)

		loc := 0
		for _, file := range pkg.GoFiles {
			loc += countLines(file)
		}
		info.LOC = loc

		result[pkg.PkgPath] = info
	}

	return result, nil
}

func countLines(file string) int {
	data, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	count := 0
	for _, b := range data {
		if b == '\n' {
			count++
		}
	}
	return count
}

func extractExportedNames(files []*ast.File) []string {
	seen := make(map[string]bool)
	var exports []string

	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		exports = append(exports, name)
	}

	for _, file := range files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Name.IsExported() {
					add(d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							add(s.Name.Name)
						}
					case *ast.ValueSpec:
						for _, name := range s.Names {
							if name.IsExported() {
								add(name.Name)
							}
						}
					}
				}
			}
		}
	}

	return exports
}
