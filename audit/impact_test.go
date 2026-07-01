package audit

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"testing"
	"path/filepath"
)

const sampleGoSrc = `package sample

type Config struct {
	Host string
	Port int
}

func NewConfig(host string) *Config {
	return &Config{Host: host}
}

func (c *Config) Validate() bool {
	return c.Host != ""
}
`

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
}

func TestAnalyzeFile_Functions(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "sample.go", sampleGoSrc)

	sm, err := AnalyzeFile(path)
	if err != nil {
		t.Fatalf("AnalyzeFile error: %v", err)
	}

	if _, ok := sm.Functions["NewConfig"]; !ok {
		t.Error("expected NewConfig in Functions")
	}
	if _, ok := sm.Functions["Validate"]; !ok {
		t.Error("expected Validate in Functions")
	}
}

func TestAnalyzeFile_Receiver(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "sample.go", sampleGoSrc)

	sm, err := AnalyzeFile(path)
	if err != nil {
		t.Fatal(err)
	}

	fbs := sm.Functions["Validate"]
	if len(fbs) == 0 {
		t.Fatal("Validate not found")
	}
	if fbs[0].Receiver != "*Config" {
		t.Errorf("expected *Config receiver, got %q", fbs[0].Receiver)
	}
	if fbs[0].StartLine == 0 {
		t.Error("StartLine must be > 0")
	}
	if fbs[0].EndLine < fbs[0].StartLine {
		t.Error("EndLine must be >= StartLine")
	}
}

func TestAnalyzeFile_Struct(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "sample.go", sampleGoSrc)

	sm, err := AnalyzeFile(path)
	if err != nil {
		t.Fatal(err)
	}

	sds := sm.Structs["Config"]
	if len(sds) == 0 {
		t.Fatal("Config struct not found")
	}
	if len(sds[0].Fields) != 2 {
		t.Errorf("expected 2 fields, got %d: %v", len(sds[0].Fields), sds[0].Fields)
	}
}

func TestAnalyzeFiles_Merge(t *testing.T) {
	dir := t.TempDir()
	p1 := writeTempFile(t, dir, "a.go", "package sample\nfunc FuncA() {}")
	p2 := writeTempFile(t, dir, "b.go", "package sample\nfunc FuncB() {}")

	sm, err := AnalyzeFiles([]string{p1, p2})
	if err != nil {
		t.Fatalf("AnalyzeFiles error: %v", err)
	}
	if _, ok := sm.Functions["FuncA"]; !ok {
		t.Error("FuncA not found")
	}
	if _, ok := sm.Functions["FuncB"]; !ok {
		t.Error("FuncB not found")
	}
}

func TestAnalyzeFile_ParseError(t *testing.T) {
	path := writeTempFile(t, t.TempDir(), "bad.go", "package bad\nfunc broken( {")
	_, err := AnalyzeFile(path)
	if err == nil {
		t.Error("expected parse error for malformed file")
	}
}

func TestAnalyzeFiles_SkipsParseErrors(t *testing.T) {
	dir := t.TempDir()
	p1 := writeTempFile(t, dir, "good.go", "package sample\nfunc Good() {}")
	p2 := writeTempFile(t, dir, "bad.go", "package bad\nfunc broken( {")

	sm, err := AnalyzeFiles([]string{p1, p2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := sm.Functions["Good"]; !ok {
		t.Error("Good function should be present from the valid file")
	}
}

// Verify the fset and ast imports are used (compile-time check via blank usage).
var _ = token.NewFileSet
var _ = parser.ParseFile
var _ = ast.File{}
