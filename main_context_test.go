package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
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
	// scoreContextFiles fallback preserves the existing candidate order.
	// resolveContextFiles passes allGoFiles here, which is deterministic because
	// allGoFiles uses git ls-files.
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

func TestExtractSignaturesReturnsOnlyDeclarationLines(t *testing.T) {
	src := []byte(strings.Join([]string{
		"package main",
		"",
		"import \"fmt\"",
		"",
		"// Doc comment",
		"func Foo() {",
		"\tfmt.Println(\"body\")",
		"}",
		"",
		"type Bar struct {",
		"\tX int",
		"}",
		"",
		"var Count int",
		"",
		"const MaxRetries = 5",
	}, "\n"))

	got := string(extractSignatures(src))
	for _, want := range []string{"func Foo() {", "type Bar struct {", "var Count int", "const MaxRetries = 5"} {
		if !strings.Contains(got, want) {
			t.Errorf("extractSignatures: expected %q in output, got:\n%s", want, got)
		}
	}
	for _, absent := range []string{"package main", "import", "fmt.Println", "X int"} {
		if strings.Contains(got, absent) {
			t.Errorf("extractSignatures: unexpected %q in output, got:\n%s", absent, got)
		}
	}
}

func TestExtractSignaturesCapsAtMaxBytesPerFile(t *testing.T) {
	// Build a source that produces many declaration lines exceeding the cap.
	var lines []string
	for i := 0; i < 300; i++ {
		lines = append(lines, fmt.Sprintf("func Func%04d() {}", i))
	}
	src := []byte(strings.Join(lines, "\n"))

	got := extractSignatures(src)
	if len(got) > maxBytesPerFile {
		t.Errorf("extractSignatures output %d bytes, want <= %d", len(got), maxBytesPerFile)
	}
}

func TestGatherFileContextTruncatesOversizedFiles(t *testing.T) {
	dir := t.TempDir()

	// Write a file larger than maxBytesPerFile with only func declarations as
	// signal — extractSignatures must return something under the cap.
	var lines []string
	for i := 0; i < 200; i++ {
		lines = append(lines, fmt.Sprintf("func Handler%04d() {}", i))
	}
	path := filepath.Join(dir, "big.go")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write big.go: %v", err)
	}

	ctx := gatherFileContext([]string{path})
	if len(ctx) == 0 {
		t.Fatal("expected non-empty context")
	}
	// The context must reference the file header.
	if !strings.Contains(ctx, "FILE: ") {
		t.Errorf("expected FILE: header in context, got:\n%s", ctx[:min(len(ctx), 200)])
	}
	// And must not exceed per-file cap plus a small overhead for the header line.
	if len(ctx) > maxBytesPerFile+200 {
		t.Errorf("gatherFileContext output %d bytes for oversized file, want <= %d", len(ctx), maxBytesPerFile+200)
	}
}

func TestTruncateFuncContextAtRuneBoundary(t *testing.T) {
	// Build a context string just over the limit using multi-byte runes.
	// prefix has maxBytesPerFile bytes of ASCII, then we add a 2-byte rune
	// to push the total over the cap.
	prefix := strings.Repeat("a", maxBytesPerFile)
	ctx := prefix + "é" // 'é' is a 2-byte UTF-8 rune, pushing total over the cap.

	got := truncateFuncContext(ctx)
	if len(got) > maxBytesPerFile+len("\n// ... (truncated)") {
		t.Errorf("truncateFuncContext returned %d bytes, want <= cap+marker", len(got))
	}
	if !strings.HasSuffix(got, "\n// ... (truncated)") {
		t.Errorf("expected truncation marker, got suffix: %q", got[max(0, len(got)-30):])
	}
	// Verify the output is valid UTF-8.
	if !utf8.ValidString(got) {
		t.Error("truncateFuncContext produced invalid UTF-8")
	}
	// The multi-byte rune must have been dropped, not partially included.
	if strings.Contains(got, "é") {
		t.Error("expected truncated rune to be absent from output")
	}
}

func TestScoreContextFilesDeterminism(t *testing.T) {
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
	recent := map[string]bool{
		"module_key_helper.go": true,
		"module_recent.go":     true,
	}

	result := scoreContextFiles(candidates, "key", history, recent)

	// Check that results are sorted by score (descending) and then by path (ascending)
	if len(result) != 4 {
		t.Errorf("Expected 4 results, got %d", len(result))
	}

	// Check that the first result has the highest score (8)
	if result[0] != "module_key_helper.go" {
		t.Errorf("Expected first result to be module_key_helper.go, got %s", result[0])
	}

	// Check that the second result has score 5
	if result[1] != "module_keyonly.go" {
		t.Errorf("Expected second result to be module_keyonly.go, got %s", result[1])
	}
}

func TestGatherFileContextTruncation(t *testing.T) {
	// Create a test file with content that exceeds maxBytesPerFile
	testFile := "test_truncation.go"
	testContent := make([]byte, maxBytesPerFile+100)
	for i := range testContent {
		testContent[i] = 'A'
	}

	// Write test content to file
	err := os.WriteFile(testFile, testContent, 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Test gatherFileContext with truncation
	context := gatherFileContext([]string{testFile})

	// Should contain signature-only fallback marker
	if !strings.Contains(context, "// ... (signatures only, file truncated due to size limit)") {
		t.Error("Expected signature-only fallback marker")
	}

	// Should not exceed maxPromptChars
	if len(context) > maxPromptChars {
		t.Errorf("Context exceeds max prompt chars: %d > %d", len(context), maxPromptChars)
	}
}

func TestEnforcePromptBudget(t *testing.T) {
	// Test with content that exceeds the limit
	longContent := strings.Repeat("A", maxPromptChars+100)

	budgeted := enforcePromptBudget(longContent)

	// Should be truncated to maxPromptChars
	if len(budgeted) != maxPromptChars {
		t.Errorf("Expected truncated content to be %d chars, got %d", maxPromptChars, len(budgeted))
	}

	// Test with content that's within limit
	shortContent := "Short content"
	budgetedShort := enforcePromptBudget(shortContent)

	// Should remain unchanged
	if budgetedShort != shortContent {
		t.Errorf("Expected short content to remain unchanged")
	}
}
