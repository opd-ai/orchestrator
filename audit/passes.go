package audit

import (
	"fmt"
	"strings"
)

func RunArchitecturePass(ctx AuditContext) []Finding {
	var findings []Finding

	for _, hotspot := range ctx.Hotspots {
		if hotspot.LOC <= 300 && hotspot.Complexity <= 15 {
			continue
		}

		severity := "medium"
		if hotspot.LOC > 500 || hotspot.Complexity > 25 {
			severity = "high"
		}

		findings = append(findings, Finding{
			Type:           "architecture_hotspot",
			Severity:       severity,
			Description:    fmt.Sprintf("%s is a hotspot with %d LOC and complexity %d", hotspot.File, hotspot.LOC, hotspot.Complexity),
			Recommendation: "Split the file or simplify control flow before adding more behavior.",
			Confidence:     0.88,
		})
	}

	for pkgPath, inbound := range ctx.CallDensity {
		if inbound > 0 || strings.HasSuffix(pkgPath, "/orchestrator") {
			continue
		}

		findings = append(findings, Finding{
			Package:        pkgPath,
			Type:           "architecture_isolated_package",
			Severity:       "medium",
			Description:    fmt.Sprintf("%s has no in-repo dependants and may be drifting from the main execution path", pkgPath),
			Recommendation: "Confirm the package is still required or add coverage that exercises it through the main workflow.",
			Confidence:     0.72,
		})
	}

	return findings
}

func RunAPIPass(ctx AuditContext) []Finding {
	var findings []Finding
	exportsByPackage := make(map[string]int)

	for _, symbol := range ctx.Exports {
		exportsByPackage[symbol.Package]++

		if symbol.Kind != "interface" {
			continue
		}

		findings = append(findings, Finding{
			Package:        symbol.Package,
			Type:           "api_exported_interface",
			Severity:       "medium",
			Description:    fmt.Sprintf("Exported interface %s expands the compatibility surface", symbol.Name),
			Recommendation: "Keep the interface minimal and verify external callers truly need it to be exported.",
			Confidence:     0.8,
		})
	}

	for pkgPath, count := range exportsByPackage {
		if count <= 8 {
			continue
		}

		findings = append(findings, Finding{
			Package:        pkgPath,
			Type:           "api_large_surface",
			Severity:       "medium",
			Description:    fmt.Sprintf("%s exposes %d exported symbols", pkgPath, count),
			Recommendation: "Review whether some symbols can stay package-private to keep the public API easier to evolve.",
			Confidence:     0.77,
		})
	}

	return findings
}

func RunConcurrencyPass(ctx AuditContext) []Finding {
	for _, imp := range ctx.Imports {
		if imp != "sync" && imp != "sync/atomic" {
			continue
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

	return nil
}
