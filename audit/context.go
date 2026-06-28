package audit

import (
	"fmt"
	"strings"
)

func BuildAuditContext(cluster Cluster, graph *DependencyGraph) AuditContext {
	var exports []SymbolInfo
	importSet := make(map[string]bool)
	callDensity := make(map[string]int)

	for _, pkgPath := range cluster.Packages {
		pkg := graph.Packages[pkgPath]

		for _, imp := range pkg.Imports {
			importSet[imp] = true
		}

		// Placeholder for call density estimation
		callDensity[pkgPath] = len(pkg.Imports)
	}

	var imports []string
	for imp := range importSet {
		imports = append(imports, imp)
	}

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
		Hotspots:       []Hotspot{},
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
