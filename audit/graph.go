package audit

func BuildDependencyGraph(pkgs map[string]*PackageInfo) *DependencyGraph {
	graph := &DependencyGraph{
		Packages: pkgs,
		Edges:    make(map[string][]string),
	}

	for path, pkg := range pkgs {
		graph.Edges[path] = pkg.Imports
	}

	return graph
}
