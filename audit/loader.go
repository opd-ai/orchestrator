package audit

import (
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
			Files:   pkg.GoFiles,
		}

		for imp := range pkg.Imports {
			info.Imports = append(info.Imports, imp)
		}

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
