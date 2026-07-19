package audit

import (
	"strings"
	"testing"
)

func cycleGraph() *DependencyGraph {
	pkgs := map[string]*PackageInfo{
		"a": {Path: "a", Imports: []string{"b"}},
		"b": {Path: "b", Imports: []string{"c"}},
		"c": {Path: "c", Imports: []string{"a"}}, // cycle: a→b→c→a
	}
	return BuildDependencyGraph(pkgs)
}

func acyclicGraph() *DependencyGraph {
	pkgs := map[string]*PackageInfo{
		"a": {Path: "a", Imports: []string{"b"}},
		"b": {Path: "b", Imports: []string{"c"}},
		"c": {Path: "c"},
	}
	return BuildDependencyGraph(pkgs)
}

func TestDetectCycles_WithCycle(t *testing.T) {
	g := cycleGraph()
	findings := g.DetectCycles()
	if len(findings) == 0 {
		t.Fatal("expected at least one cycle finding")
	}
	for _, f := range findings {
		if f.Type != "architecture_dependency_cycle" {
			t.Errorf("unexpected finding type: %s", f.Type)
		}
		if f.Severity != "high" {
			t.Errorf("expected high severity, got %s", f.Severity)
		}
		if !strings.Contains(f.Description, "Dependency cycle") {
			t.Errorf("description should mention cycle: %s", f.Description)
		}
	}
}

func TestDetectCycles_NoCycle(t *testing.T) {
	g := acyclicGraph()
	findings := g.DetectCycles()
	if len(findings) != 0 {
		t.Errorf("expected no cycle findings, got %d: %v", len(findings), findings)
	}
}

func TestCheckLayering_Violation(t *testing.T) {
	pkgs := map[string]*PackageInfo{
		"cmd": {Path: "cmd", Imports: []string{"lib"}},
		"lib": {Path: "lib", Imports: []string{"cmd"}}, // lib imports cmd: layering violation
	}
	g := BuildDependencyGraph(pkgs)
	layers := [][]string{{"cmd"}, {"lib"}}
	findings := g.CheckLayering(layers)
	if len(findings) == 0 {
		t.Fatal("expected layering violation finding")
	}
	found := false
	for _, f := range findings {
		if f.Type == "architecture_layering_violation" && strings.Contains(f.Description, "lib") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected layering violation for lib, got: %v", findings)
	}
}

func TestCheckLayering_NoViolation(t *testing.T) {
	pkgs := map[string]*PackageInfo{
		"cmd": {Path: "cmd", Imports: []string{"lib"}},
		"lib": {Path: "lib"},
	}
	g := BuildDependencyGraph(pkgs)
	layers := [][]string{{"cmd"}, {"lib"}}
	findings := g.CheckLayering(layers)
	if len(findings) != 0 {
		t.Errorf("expected no layering violations, got %d", len(findings))
	}
}

func TestRunArchitectureGraphChecks_EmptyLayers(t *testing.T) {
	g := acyclicGraph()
	findings := RunArchitectureGraphChecks(g, nil)
	// No cycles, no layers provided — should be empty.
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}
