package audit

import (
	"fmt"
	"sort"
)

// ClusterPackages groups connected in-repo packages into deterministic audit clusters.
func ClusterPackages(graph *DependencyGraph) []Cluster {
	visited := make(map[string]bool)
	var clusters []Cluster
	clusterID := 0

	// Sort package keys for deterministic cluster assignment across runs (F-16).
	keys := make([]string, 0, len(graph.Packages))
	for pkg := range graph.Packages {
		keys = append(keys, pkg)
	}
	sort.Strings(keys)

	for _, pkg := range keys {
		if visited[pkg] {
			continue
		}
		group, totalLOC := collectCluster(graph, pkg, visited)

		// Use fmt.Sprintf to produce decimal IDs ("cluster_0", "cluster_1", …) instead
		// of converting the integer as a Unicode code point (F-03).
		clusters = append(clusters, Cluster{
			ID:       fmt.Sprintf("cluster_%d", clusterID),
			Packages: group,
			TotalLOC: totalLOC,
		})
		clusterID++
	}

	return clusters
}

// collectCluster walks dependencies from a root package and returns the cluster members and total LOC.
func collectCluster(graph *DependencyGraph, root string, visited map[string]bool) ([]string, int) {
	stack := []string{root}
	var group []string
	totalLOC := 0

	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if visited[cur] {
			continue
		}

		pkgInfo, ok := graph.Packages[cur]
		if !ok {
			continue
		}

		visited[cur] = true
		group = append(group, cur)
		totalLOC += pkgInfo.LOC
		stack = appendUnvisitedDeps(stack, graph.Edges[cur], visited)
	}

	return group, totalLOC
}

// appendUnvisitedDeps pushes unseen dependency nodes onto the traversal stack.
func appendUnvisitedDeps(stack, deps []string, visited map[string]bool) []string {
	for _, dep := range deps {
		if !visited[dep] {
			stack = append(stack, dep)
		}
	}
	return stack
}
