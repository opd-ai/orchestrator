package main

import (
	"strings"
	"testing"
)

func TestClassifyBuildFailure(t *testing.T) {
	cases := map[string]string{
		"./main.go:10:2: undefined: foo":                    "undefined symbol",
		"./main.go:10:2: imported and not used: \"fmt\"":    "unused import",
		"./main.go:10:2: undefined: fmt.Println":            "missing import",
		"./main.go:10:2: cannot use x (type int) as string": "type mismatch",
		"./main.go:10:2: value redeclared in this block":    "redeclaration",
	}

	for input, want := range cases {
		if got := classifyBuildFailure(input); got != want {
			t.Fatalf("classifyBuildFailure(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestMostCommonFailure(t *testing.T) {
	failures := map[string]int{
		"undefined symbol": 3,
		"unused import":    1,
	}

	if got := mostCommonFailure(failures); got != "undefined symbol" {
		t.Fatalf("mostCommonFailure() = %q, want %q", got, "undefined symbol")
	}
}

func TestBuildFixHints(t *testing.T) {
	buildOut := `
./main.go:10:2: undefined: fmt.Println
./main.go:11:2: imported and not used: "errors"
`

	hints := buildFixHints(buildOut)

	checks := []string{
		"category: missing import",
		"category: unused import",
		"hint: Add the missing import",
		"hint: Remove unused imports",
	}
	for _, check := range checks {
		if !strings.Contains(hints, check) {
			t.Fatalf("buildFixHints() missing %q in %q", check, hints)
		}
	}
}
