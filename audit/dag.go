package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
)

// CallEdge represents a directed function call relationship.
type CallEdge struct {
	Caller string
	Callee string
	File   string
}

// FuncDAG is a directed acyclic approximation of function call relationships
// within a set of Go source files.
type FuncDAG struct {
	Edges   []CallEdge
	Callers map[string][]string // callee -> callers
	Callees map[string][]string // caller -> callees
}

// BuildFuncDAG builds a function-level call graph from the provided Go source files.
func BuildFuncDAG(paths []string) (*FuncDAG, error) {
	dag := &FuncDAG{
		Callers: make(map[string][]string),
		Callees: make(map[string][]string),
	}
	for _, path := range paths {
		addFileToDAG(dag, path)
	}
	return dag, nil
}

// TopologicalOrder returns functions sorted so that callees appear before their callers.
// This gives an implementation ordering: define helper functions before the functions that use them.
func (dag *FuncDAG) TopologicalOrder() []string {
	inDegree, allNodes := dag.computeInDegrees()
	queue := nodesWithZeroInDegree(inDegree, allNodes)
	return topoProcess(queue, dag.Callers, inDegree)
}

// computeInDegrees returns each function's in-degree (number of callees it depends on)
// and the set of all nodes observed in the DAG.
func (dag *FuncDAG) computeInDegrees() (map[string]int, map[string]bool) {
	inDegree := make(map[string]int)
	allNodes := make(map[string]bool)
	for caller, callees := range dag.Callees {
		allNodes[caller] = true
		inDegree[caller] = len(callees)
		for _, callee := range callees {
			allNodes[callee] = true
		}
	}
	return inDegree, allNodes
}

// nodesWithZeroInDegree returns all nodes with in-degree zero (no dependencies),
// which serve as the starting points for topological processing.
// The returned slice is sorted to ensure deterministic ordering.
func nodesWithZeroInDegree(inDegree map[string]int, allNodes map[string]bool) []string {
	var queue []string
	for fn := range allNodes {
		if inDegree[fn] == 0 {
			queue = append(queue, fn)
		}
	}
	sort.Strings(queue)
	return queue
}

func topoProcess(queue []string, callers map[string][]string, inDegree map[string]int) []string {
	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)
		var newNodes []string
		for _, caller := range callers[node] {
			inDegree[caller]--
			if inDegree[caller] == 0 {
				newNodes = append(newNodes, caller)
			}
		}
		sort.Strings(newNodes)
		queue = append(queue, newNodes...)
	}
	return order
}

func addFileToDAG(dag *FuncDAG, path string) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return
	}
	for _, decl := range node.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		// Register every function in Callees so that functions making no calls
		// still appear in TopologicalOrder.
		name := fd.Name.Name
		if _, exists := dag.Callees[name]; !exists {
			dag.Callees[name] = []string{}
		}
		collectCalls(dag, fd, path)
	}
}

func collectCalls(dag *FuncDAG, fd *ast.FuncDecl, path string) {
	caller := fd.Name.Name
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee := callName(call)
		if callee == "" || callee == caller {
			return true
		}
		edge := CallEdge{Caller: caller, Callee: callee, File: path}
		dag.Edges = append(dag.Edges, edge)
		dag.Callees[caller] = appendUniq(dag.Callees[caller], callee)
		dag.Callers[callee] = appendUniq(dag.Callers[callee], caller)
		return true
	})
}

func callName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	default:
		return ""
	}
}

func appendUniq(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
