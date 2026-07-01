package audit

import (
	"fmt"
	"strings"
)

func RunArchitecturePass(ctx AuditContext) []Finding {
	findings := architectureHotspotFindings(ctx.Hotspots)
	return append(findings, isolatedPackageFindings(ctx.CallDensity)...)
}

func RunAPIPass(ctx AuditContext) []Finding {
	findings := apiInterfaceFindings(ctx.Exports)
	return append(findings, apiSurfaceFindings(ctx.Exports)...)
}

func RunConcurrencyPass(ctx AuditContext) []Finding {
	return concurrencyFindings(ctx.Imports)
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

func firstConcurrencyImport(imports []string) (string, bool) {
	for _, imp := range imports {
		if imp == "sync" || imp == "sync/atomic" {
			return imp, true
		}
	}
	return "", false
}

func concurrencyFindings(imports []string) []Finding {
	imp, ok := firstConcurrencyImport(imports)
	if !ok {
		return nil
	}

	return []Finding{
		{
			Type:           "concurrency_primitive_usage",
			Severity:       "medium",
			Description:    fmt.Sprintf("Cluster imports %s and should be reviewed for lock scope and goroutine safety", imp),
			Recommendation: "Audit synchronization paths and add targeted tests around concurrent access.",
			Confidence:     0.75,
		},
	}
}
