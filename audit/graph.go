package audit

import (
	"fmt"
	"sort"
	"strings"
)

// BuildDependencyGraph constructs a dependency graph from the loaded package set.
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

// DetectCycles performs a DFS-based cycle detection over the dependency graph.
// It returns one Finding per cycle found.  Only edges between packages present
// in graph.Packages are considered (stdlib / external imports are skipped).
func (g *DependencyGraph) DetectCycles() []Finding {
	state := &cycleState{
		graph:   g,
		visited: make(map[string]bool),
		onStack: make(map[string]bool),
	}
	roots := sortedPackageKeys(g.Packages)
	for _, root := range roots {
		if !state.visited[root] {
			state.dfs(root, nil)
		}
	}
	return cycleFindings(state.cycles)
}

type cycleState struct {
	graph   *DependencyGraph
	visited map[string]bool
	onStack map[string]bool
	cycles  [][]string
}

func (s *cycleState) dfs(path string, stack []string) {
	s.visited[path] = true
	s.onStack[path] = true
	stack = append(stack, path)

	for _, dep := range s.graph.Edges[path] {
		if _, inRepo := s.graph.Packages[dep]; !inRepo {
			continue
		}
		if s.onStack[dep] {
			s.cycles = append(s.cycles, extractCycle(stack, dep))
			continue
		}
		if !s.visited[dep] {
			s.dfs(dep, stack)
		}
	}
	s.onStack[path] = false
}

func extractCycle(stack []string, start string) []string {
	for i, p := range stack {
		if p == start {
			cycle := make([]string, len(stack[i:]))
			copy(cycle, stack[i:])
			return cycle
		}
	}
	return nil
}

func sortedPackageKeys(pkgs map[string]*PackageInfo) []string {
	keys := make([]string, 0, len(pkgs))
	for k := range pkgs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func cycleFindings(cycles [][]string) []Finding {
	var findings []Finding
	for _, cycle := range cycles {
		if len(cycle) == 0 {
			continue
		}
		// Append the starting node to close the cycle path visually (e.g. "a → b → c → a").
		display := make([]string, len(cycle)+1)
		copy(display, cycle)
		display[len(cycle)] = cycle[0]
		path := strings.Join(display, " → ")
		findings = append(findings, Finding{
			Type:           "architecture_dependency_cycle",
			Severity:       "high",
			Description:    fmt.Sprintf("Dependency cycle detected: %s", path),
			Recommendation: "Break the cycle by introducing an interface or moving shared types to a lower-level package.",
			Confidence:     0.95,
		})
	}
	return findings
}

// DeriveLayersFromGraph assigns each package to a topological layer using
// Kahn's algorithm.  Packages with no in-repo dependencies form the innermost
// layer (foundation); packages that depend on them form the next layer, and so
// on.  The returned slice is ordered from outermost (layers[0], e.g. "main") to
// innermost (layers[len-1], foundation), matching the convention expected by
// CheckLayering.  Packages that are part of a dependency cycle are excluded
// (they will be reported separately by DetectCycles).
func DeriveLayersFromGraph(g *DependencyGraph) [][]string {
	// Build an in-repo dependency count and reverse-edge map for Kahn's algorithm.
	depsCount := make(map[string]int, len(g.Packages))
	dependents := make(map[string][]string) // dep → packages that import dep

	for pkg := range g.Packages {
		depsCount[pkg] = 0
	}
	for pkg, imports := range g.Edges {
		if _, ok := g.Packages[pkg]; !ok {
			continue
		}
		for _, dep := range imports {
			if _, ok := g.Packages[dep]; !ok {
				continue
			}
			depsCount[pkg]++
			dependents[dep] = append(dependents[dep], pkg)
		}
	}

	// Seed the queue with packages that have no in-repo dependencies.
	var queue []string
	for pkg := range g.Packages {
		if depsCount[pkg] == 0 {
			queue = append(queue, pkg)
		}
	}
	sort.Strings(queue)

	// BFS level-by-level to build layers (innermost first).
	var layers [][]string
	visited := make(map[string]bool, len(g.Packages))
	for len(queue) > 0 {
		sort.Strings(queue)
		layers = append(layers, append([]string{}, queue...))
		var next []string
		for _, pkg := range queue {
			visited[pkg] = true
			for _, dep := range dependents[pkg] {
				depsCount[dep]--
				if depsCount[dep] == 0 && !visited[dep] {
					next = append(next, dep)
				}
			}
		}
		queue = next
	}

	// Reverse so that layers[0] is outermost (most dependent, e.g. main/cmd).
	for i, j := 0, len(layers)-1; i < j; i, j = i+1, j-1 {
		layers[i], layers[j] = layers[j], layers[i]
	}
	return layers
}

// CheckLayering verifies that packages in lower layers (higher index in layers)
// do not import packages from upper layers (lower index).  layers[0] is the
// top-most layer (e.g. "cmd" or "main"); layers[len-1] is the innermost.
// Returns findings for each detected layering violation.
func (g *DependencyGraph) CheckLayering(layers [][]string) []Finding {
	rank := buildLayerRank(layers)
	var findings []Finding

	// Deterministic order.
	paths := make([]string, 0, len(g.Packages))
	for path := range g.Packages {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		importer, ok := rank[path]
		if !ok {
			continue
		}
		for _, dep := range g.Edges[path] {
			importee, ok := rank[dep]
			if !ok {
				continue
			}
			if importee < importer {
				findings = append(findings, Finding{
					Package:        path,
					Type:           "architecture_layering_violation",
					Severity:       "high",
					Description:    fmt.Sprintf("%s (layer %d) imports %s (layer %d), violating layering rules", path, importer, dep, importee),
					Recommendation: "Restructure so that lower-level packages do not depend on higher-level ones.",
					Confidence:     0.90,
				})
			}
		}
	}
	return findings
}

// buildLayerRank maps each package in layers to its layer index (0 = topmost).
func buildLayerRank(layers [][]string) map[string]int {
	rank := make(map[string]int)
	for i, layer := range layers {
		for _, pkg := range layer {
			rank[pkg] = i
		}
	}
	return rank
}
