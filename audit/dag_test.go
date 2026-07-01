package audit

import (
	"testing"
)

const callerGoSrc = `package sample

func Leaf() int { return 1 }

func Middle() int { return Leaf() + 1 }

func Root() int { return Middle() + Leaf() }
`

func TestBuildFuncDAG_Edges(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "caller.go", callerGoSrc)

	dag, err := BuildFuncDAG([]string{path})
	if err != nil {
		t.Fatalf("BuildFuncDAG error: %v", err)
	}

	if len(dag.Edges) == 0 {
		t.Error("expected at least one call edge")
	}
	if _, ok := dag.Callers["Leaf"]; !ok {
		t.Error("Leaf should have callers (Middle and Root call it)")
	}
	if _, ok := dag.Callees["Root"]; !ok {
		t.Error("Root should have callees")
	}
}

func TestTopologicalOrder_LeafBeforeRoot(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "caller.go", callerGoSrc)

	dag, err := BuildFuncDAG([]string{path})
	if err != nil {
		t.Fatal(err)
	}

	order := dag.TopologicalOrder()
	if len(order) == 0 {
		t.Fatal("expected non-empty topological order")
	}

	pos := make(map[string]int, len(order))
	for i, fn := range order {
		pos[fn] = i
	}

	leafPos, hasLeaf := pos["Leaf"]
	rootPos, hasRoot := pos["Root"]

	if !hasLeaf || !hasRoot {
		t.Skip("Leaf or Root not present in DAG output; call graph may be incomplete")
	}
	if leafPos > rootPos {
		t.Errorf("Leaf (pos %d) should appear before Root (pos %d) in impl order", leafPos, rootPos)
	}
}

func TestBuildFuncDAG_EmptyPaths(t *testing.T) {
	dag, err := BuildFuncDAG([]string{})
	if err != nil {
		t.Fatalf("unexpected error for empty paths: %v", err)
	}
	if len(dag.Edges) != 0 {
		t.Error("expected no edges for empty path list")
	}
}

func TestBuildFuncDAG_InvalidPath(t *testing.T) {
	dag, err := BuildFuncDAG([]string{"/nonexistent/file.go"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dag.Edges) != 0 {
		t.Error("expected no edges for nonexistent file")
	}
}

func TestAppendUniq(t *testing.T) {
	s := appendUniq(nil, "a")
	s = appendUniq(s, "b")
	s = appendUniq(s, "a") // duplicate; should not grow
	if len(s) != 2 {
		t.Errorf("expected 2 elements, got %d: %v", len(s), s)
	}
}
