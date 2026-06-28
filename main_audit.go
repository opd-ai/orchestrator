package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/opd-ai/orchestrator/audit"
)

func runAuditMode() {
	start := time.Now()

	if auditPattern == "" {
		auditPattern = "./..."
	}

	fmt.Println("Loading packages...")
	pkgs, err := audit.LoadPackages(auditPattern)
	exitOnErr(err)

	fmt.Println("Building dependency graph...")
	graph := audit.BuildDependencyGraph(pkgs)

	fmt.Println("Clustering packages...")
	clusters := audit.ClusterPackages(graph)

	var allFindings []audit.Finding

	for _, cluster := range clusters {
		ctx := audit.BuildAuditContext(cluster, graph)

		findings := runAuditPasses(ctx)

		// Attach package info to findings
		for i := range findings {
			if findings[i].Package == "" && len(cluster.Packages) > 0 {
				findings[i].Package = cluster.Packages[0]
			}
		}

		allFindings = append(allFindings, findings...)
	}

	if auditOutput == "" {
		auditOutput = "audit_findings.json"
	}

	fmt.Println("Saving findings...")
	exitOnErr(audit.SaveFindings(auditOutput, allFindings))

	fmt.Printf("Audit complete in %s\n", time.Since(start))
}

func runAuditPasses(ctx audit.AuditContext) []audit.Finding {
	var findings []audit.Finding

	switch strings.ToLower(auditPass) {

	case "architecture":
		findings = append(findings, audit.RunArchitecturePass(ctx)...)

	case "api":
		findings = append(findings, audit.RunAPIPass(ctx)...)

	case "concurrency":
		findings = append(findings, audit.RunConcurrencyPass(ctx)...)

	case "all", "":
		findings = append(findings,
			audit.RunArchitecturePass(ctx)...,
		)
		findings = append(findings,
			audit.RunAPIPass(ctx)...,
		)
		findings = append(findings,
			audit.RunConcurrencyPass(ctx)...,
		)

	default:
		fmt.Println("Unknown audit pass:", auditPass)
		os.Exit(1)
	}

	return findings
}
