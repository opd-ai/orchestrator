package audit

import (
	"fmt"
	"sort"
	"strings"
)

func RunArchitecturePass(ctx AuditContext) []Finding {
	findings := architectureHotspotFindings(ctx.Hotspots)
	findings = append(findings, isolatedPackageFindings(ctx.CallDensity)...)
	findings = append(findings, deadFunctionFindings(ctx.DeadFunctions)...)
	findings = append(findings, highCentralityFindings(ctx.FuncDAG)...)
	return findings
}

// RunArchitectureGraphChecks performs graph-level architecture checks that
// require the full dependency graph rather than a single cluster context.
// It detects dependency cycles and, when explicit layer definitions are
// provided, reports package layering violations.
func RunArchitectureGraphChecks(graph *DependencyGraph, layers [][]string) []Finding {
	findings := graph.DetectCycles()
	if len(layers) > 0 {
		findings = append(findings, graph.CheckLayering(layers)...)
	}
	return findings
}

func deadFunctionFindings(names []string) []Finding {
	if len(names) == 0 {
		return nil
	}
	return []Finding{{
		Type:           "dead_code_candidate",
		Severity:       "low",
		Description:    fmt.Sprintf("%d unexported functions appear unreferenced: %v", len(names), names),
		Recommendation: "Verify these functions are unused and remove them to reduce codebase size.",
		Confidence:     0.65,
	}}
}

// highCentralityFindings uses the function-level DAG to flag functions with an
// unusually high number of callers.  Such functions are high-risk modification
// targets because a change propagates to many callers.
func highCentralityFindings(dag *FuncDAG) []Finding {
	const callerThreshold = 4
	if dag == nil {
		return nil
	}
	// Collect and sort function names for deterministic output order.
	fns := make([]string, 0, len(dag.Callers))
	for fn := range dag.Callers {
		fns = append(fns, fn)
	}
	sort.Strings(fns)
	var findings []Finding
	for _, fn := range fns {
		callers := dag.Callers[fn]
		if len(callers) < callerThreshold {
			continue
		}
		findings = append(findings, Finding{
			Type:           "architecture_high_centrality_function",
			Severity:       "medium",
			Description:    fmt.Sprintf("Function %q is called by %d callers — changes carry high propagation risk", fn, len(callers)),
			Recommendation: "Treat changes to this function with extra care; add tests for each known caller.",
			Confidence:     0.80,
		})
	}
	return findings
}

func RunAPIPass(ctx AuditContext) []Finding {
	findings := apiInterfaceFindings(ctx.Exports)
	findings = append(findings, apiSurfaceFindings(ctx.Exports)...)
	findings = append(findings, undocumentedExportFindings(ctx.Exports)...)
	findings = append(findings, interfaceDriftFindings(ctx.Exports)...)
	return findings
}

func architectureHotspotFinding(hotspot Hotspot) (Finding, bool) {
	if hotspot.LOC <= 300 && hotspot.Complexity <= 15 {
		return Finding{}, false
	}

	severity := "medium"
	if hotspot.LOC > 500 || hotspot.Complexity > 25 {
		severity = "high"
	}

	return Finding{
		Type:           "architecture_hotspot",
		Severity:       severity,
		Description:    fmt.Sprintf("%s is a hotspot with %d LOC and complexity %d", hotspot.File, hotspot.LOC, hotspot.Complexity),
		Recommendation: "Split the file or simplify control flow before adding more behavior.",
		Confidence:     0.88,
	}, true
}

func architectureHotspotFindings(hotspots []Hotspot) []Finding {
	var findings []Finding
	for _, hotspot := range hotspots {
		if finding, ok := architectureHotspotFinding(hotspot); ok {
			findings = append(findings, finding)
		}
	}
	return findings
}

func isolatedPackageFinding(pkgPath string, inbound int) (Finding, bool) {
	if inbound > 0 || strings.HasSuffix(pkgPath, "/orchestrator") {
		return Finding{}, false
	}

	return Finding{
		Package:        pkgPath,
		Type:           "architecture_isolated_package",
		Severity:       "medium",
		Description:    fmt.Sprintf("%s has no in-repo dependants and may be drifting from the main execution path", pkgPath),
		Recommendation: "Confirm the package is still required or add coverage that exercises it through the main workflow.",
		Confidence:     0.72,
	}, true
}

func isolatedPackageFindings(callDensity map[string]int) []Finding {
	var findings []Finding
	for pkgPath, inbound := range callDensity {
		if finding, ok := isolatedPackageFinding(pkgPath, inbound); ok {
			findings = append(findings, finding)
		}
	}
	return findings
}

func apiInterfaceFinding(symbol SymbolInfo) (Finding, bool) {
	if symbol.Kind != "interface" {
		return Finding{}, false
	}

	return Finding{
		Package:        symbol.Package,
		Type:           "api_exported_interface",
		Severity:       "medium",
		Description:    fmt.Sprintf("Exported interface %s expands the compatibility surface", symbol.Name),
		Recommendation: "Keep the interface minimal and verify external callers truly need it to be exported.",
		Confidence:     0.8,
	}, true
}

func apiInterfaceFindings(exports []SymbolInfo) []Finding {
	var findings []Finding
	for _, symbol := range exports {
		if finding, ok := apiInterfaceFinding(symbol); ok {
			findings = append(findings, finding)
		}
	}
	return findings
}

func apiSurfaceFinding(pkgPath string, count int) (Finding, bool) {
	if count <= 8 {
		return Finding{}, false
	}

	return Finding{
		Package:        pkgPath,
		Type:           "api_large_surface",
		Severity:       "medium",
		Description:    fmt.Sprintf("%s exposes %d exported symbols", pkgPath, count),
		Recommendation: "Review whether some symbols can stay package-private to keep the public API easier to evolve.",
		Confidence:     0.77,
	}, true
}

func apiSurfaceFindings(exports []SymbolInfo) []Finding {
	exportsByPackage := make(map[string]int)
	for _, symbol := range exports {
		exportsByPackage[symbol.Package]++
	}

	var findings []Finding
	for pkgPath, count := range exportsByPackage {
		if finding, ok := apiSurfaceFinding(pkgPath, count); ok {
			findings = append(findings, finding)
		}
	}
	return findings
}

// undocumentedExportFinding returns a finding when an exported symbol lacks a doc comment.
func undocumentedExportFinding(symbol SymbolInfo) (Finding, bool) {
	if symbol.HasDoc {
		return Finding{}, false
	}
	return Finding{
		Package:        symbol.Package,
		Type:           "api_undocumented_export",
		Severity:       "low",
		Description:    fmt.Sprintf("Exported %s %s has no doc comment", symbol.Kind, symbol.Name),
		Recommendation: "Add a doc comment to all exported symbols to satisfy Go documentation conventions.",
		Confidence:     0.90,
	}, true
}

func undocumentedExportFindings(exports []SymbolInfo) []Finding {
	var findings []Finding
	for _, sym := range exports {
		if f, ok := undocumentedExportFinding(sym); ok {
			findings = append(findings, f)
		}
	}
	return findings
}

// interfaceDriftFinding flags an exported interface when none of the exported
// receiver methods in the same symbol set implement all of its declared methods.
// This is a best-effort heuristic: it only considers methods visible in the
// same audit cluster, so false positives are possible for cross-package consumers.
func interfaceDriftFinding(iface SymbolInfo, exports []SymbolInfo) (Finding, bool) {
	if iface.Kind != "interface" || len(iface.Methods) == 0 {
		return Finding{}, false
	}
	if hasConcreteImplementor(iface, exports) {
		return Finding{}, false
	}
	return Finding{
		Package:        iface.Package,
		Type:           "api_interface_drift",
		Severity:       "medium",
		Description:    fmt.Sprintf("Exported interface %s has no in-cluster concrete implementor; it may have drifted from the codebase", iface.Name),
		Recommendation: "Ensure at least one concrete type in the package implements the interface, or restrict its visibility.",
		Confidence:     0.70,
	}, true
}

// hasConcreteImplementor reports whether any exported method set in exports
// covers every method name declared in iface.
func hasConcreteImplementor(iface SymbolInfo, exports []SymbolInfo) bool {
	// Build a set of method names per receiver.
	receiverMethods := make(map[string]map[string]bool)
	for _, sym := range exports {
		if sym.Kind != "method" || sym.Receiver == "" {
			continue
		}
		base := strings.TrimPrefix(sym.Receiver, "*")
		if receiverMethods[base] == nil {
			receiverMethods[base] = make(map[string]bool)
		}
		receiverMethods[base][sym.Name] = true
	}

	for _, methods := range receiverMethods {
		if implementsAll(iface.Methods, methods) {
			return true
		}
	}
	return false
}

// implementsAll reports whether methodSet contains every name in required.
func implementsAll(required []string, methodSet map[string]bool) bool {
	for _, m := range required {
		if !methodSet[m] {
			return false
		}
	}
	return true
}

func interfaceDriftFindings(exports []SymbolInfo) []Finding {
	var findings []Finding
	for _, sym := range exports {
		if f, ok := interfaceDriftFinding(sym, exports); ok {
			findings = append(findings, f)
		}
	}
	return findings
}
