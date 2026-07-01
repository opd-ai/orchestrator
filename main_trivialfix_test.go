package main

import (
	"slices"
	"testing"
)

func TestGoFilesFromContext(t *testing.T) {
	files := goFilesFromContext([]string{"main.go", "README.md", "audit/graph.go"})
	want := []string{"main.go", "audit/graph.go"}
	if !slices.Equal(files, want) {
		t.Fatalf("goFilesFromContext() = %v, want %v", files, want)
	}
}

func TestClassifyCompilerIssues(t *testing.T) {
	buildOut := `
./main.go:10:2: imported and not used: "fmt"
./main.go:11:2: undefined: fmt.Println
`
	issues := classifyCompilerIssues(buildOut)
	want := []string{"missing import", "unused import"}
	if !slices.Equal(issues, want) {
		t.Fatalf("classifyCompilerIssues() = %v, want %v", issues, want)
	}
}

func TestShouldRunGoimports(t *testing.T) {
	if !shouldRunGoimports([]string{"unused import"}) {
		t.Fatal("expected goimports for unused import")
	}
	if !shouldRunGoimports([]string{"missing import"}) {
		t.Fatal("expected goimports for missing import")
	}
	if shouldRunGoimports([]string{"type mismatch"}) {
		t.Fatal("did not expect goimports for type mismatch")
	}
}
