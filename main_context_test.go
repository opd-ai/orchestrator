package main

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

func TestContextFileWeightScoring(t *testing.T) {
	history := map[string]bool{
		"history.go":    true,
		"keyhistory.go": true,
	}
	recent := map[string]bool{"recent.go": true}

	cases := []struct {
		path string
		kw   string
		want int
	}{
		{"keyonly.go", "key", 5},    // keyword match only
		{"history.go", "other", 3},  // history match only
		{"recent.go", "other", 1},   // recency match only
		{"keyhistory.go", "key", 8}, // keyword(5) + history(3)
		{"nothing.go", "key", 0},    // no match
		{"keyonly.go", "", 0},       // empty keyword → no keyword score
	}

	for _, tc := range cases {
		got := contextFileWeight(tc.path, tc.kw, history, recent)
		if got != tc.want {
			t.Errorf("contextFileWeight(%q, %q) = %d, want %d", tc.path, tc.kw, got, tc.want)
		}
	}
}

func TestScoreContextFilesRankingOrder(t *testing.T) {
	candidates := []string{
		"module_key_helper.go", // keyword(5) + history(3) = 8
		"module_keyonly.go",    // keyword(5) = 5
		"module_history.go",    // history(3) = 3
		"module_recent.go",     // recency(1) = 1
		"module_nothing.go",    // 0 → excluded
	}
	history := map[string]bool{
		"module_key_helper.go": true,
		"module_history.go":    true,
	}
	recent := map[string]bool{"module_recent.go": true}

	got := scoreContextFiles(candidates, "key", history, recent)
	want := []string{
		"module_key_helper.go",
		"module_keyonly.go",
		"module_history.go",
		"module_recent.go",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("scoreContextFiles() = %v, want %v", got, want)
	}
}

func TestScoreContextFilesTiesBrokenLexically(t *testing.T) {
	// b_key.go and a_key.go both score 5; a comes first lexically.
	candidates := []string{"b_key.go", "a_key.go", "unrelated.go"}
	got := scoreContextFiles(candidates, "key", nil, nil)

	if len(got) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(got))
	}
	if got[0] != "a_key.go" || got[1] != "b_key.go" {
		t.Fatalf("expected lexical tie-break: %v", got)
	}
}

func TestScoreContextFilesDeterministic(t *testing.T) {
	candidates := []string{"b_key.go", "a_key.go", "history.go", "recent.go"}
	history := map[string]bool{"history.go": true}
	recent := map[string]bool{"recent.go": true}

	got1 := scoreContextFiles(candidates, "key", history, recent)
	got2 := scoreContextFiles(candidates, "key", history, recent)

	if !slices.Equal(got1, got2) {
		t.Fatalf("non-deterministic results: %v vs %v", got1, got2)
	}
}

func TestScoreContextFilesCappedAtMax(t *testing.T) {
	// Generate more than maxContextFiles files all with keyword matches.
	candidates := make([]string, maxContextFiles+3)
	for i := range candidates {
		candidates[i] = fmt.Sprintf("z_key_%02d.go", i)
	}

	got := scoreContextFiles(candidates, "key", nil, nil)
	if len(got) != maxContextFiles {
		t.Fatalf("expected %d files capped, got %d", maxContextFiles, len(got))
	}
}

func TestScoreContextFilesEmptyKeyword(t *testing.T) {
	// With no keyword, only history/recency matches contribute.
	candidates := []string{"history.go", "recent.go", "neither.go"}
	history := map[string]bool{"history.go": true}
	recent := map[string]bool{"recent.go": true}

	got := scoreContextFiles(candidates, "", history, recent)
	want := []string{"history.go", "recent.go"}
	if !slices.Equal(got, want) {
		t.Fatalf("scoreContextFiles(empty kw) = %v, want %v", got, want)
	}
}

func TestScoreContextFilesFallsBackWhenNothingScores(t *testing.T) {
	candidates := []string{
		"zeta.go",
		"alpha.go",
		"beta.go",
		"gamma.go",
		"delta.go",
		"epsilon.go",
	}

	got := scoreContextFiles(candidates, "", nil, nil)
	want := candidates[:maxContextFiles]
	if !slices.Equal(got, want) {
		t.Fatalf("scoreContextFiles(no scores) = %v, want %v", got, want)
	}
}

func TestLoadRecentFilesSetCachesResult(t *testing.T) {
	originalOutput := gitRecentFilesOutput
	originalCache := cachedRecentFilesSet
	originalLoaded := cachedRecentFilesSetLoaded
	t.Cleanup(func() {
		gitRecentFilesOutput = originalOutput
		cachedRecentFilesSet = originalCache
		cachedRecentFilesSetLoaded = originalLoaded
	})

	cachedRecentFilesSet = nil
	cachedRecentFilesSetLoaded = false

	calls := 0
	gitRecentFilesOutput = func() ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte("first.go\nnotes.txt\n"), nil
		}
		return []byte("second.go\n"), nil
	}

	first := loadRecentFilesSet()
	second := loadRecentFilesSet()

	if calls != 1 {
		t.Fatalf("gitRecentFilesOutput called %d times, want 1", calls)
	}
	if !first["first.go"] || !second["first.go"] {
		t.Fatalf("expected cached result to contain first.go: first=%v second=%v", first, second)
	}
	if first["second.go"] || second["second.go"] {
		t.Fatalf("expected cached result to ignore later output: first=%v second=%v", first, second)
	}
}

func TestLoadRecentFilesSetRetriesAfterFailure(t *testing.T) {
	originalOutput := gitRecentFilesOutput
	originalCache := cachedRecentFilesSet
	originalLoaded := cachedRecentFilesSetLoaded
	t.Cleanup(func() {
		gitRecentFilesOutput = originalOutput
		cachedRecentFilesSet = originalCache
		cachedRecentFilesSetLoaded = originalLoaded
	})

	cachedRecentFilesSet = nil
	cachedRecentFilesSetLoaded = false

	calls := 0
	gitRecentFilesOutput = func() ([]byte, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("git log failed")
		}
		return []byte("retry.go\n"), nil
	}

	first := loadRecentFilesSet()
	second := loadRecentFilesSet()

	if first != nil {
		t.Fatalf("expected nil result on first failure, got %v", first)
	}
	if calls != 2 {
		t.Fatalf("gitRecentFilesOutput called %d times, want 2", calls)
	}
	if !second["retry.go"] {
		t.Fatalf("expected retry.go after retry, got %v", second)
	}
}
