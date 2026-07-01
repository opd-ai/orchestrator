package audit

func ClusterPackages(graph *DependencyGraph) []Cluster {
	visited := make(map[string]bool)
	var clusters []Cluster
	clusterID := 0

	for pkg := range graph.Packages {
		if visited[pkg] {
			continue
		}
		group, totalLOC := collectCluster(graph, pkg, visited)

		clusters = append(clusters, Cluster{
			ID:       "cluster_" + string(rune(clusterID)),
			Packages: group,
			TotalLOC: totalLOC,
		})
		clusterID++
	}

	return clusters
}

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

func appendUnvisitedDeps(stack, deps []string, visited map[string]bool) []string {
	for _, dep := range deps {
		if !visited[dep] {
			stack = append(stack, dep)
		}
	}
	return stack
}
