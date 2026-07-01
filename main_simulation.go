package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// simulatePatch applies the diff to a temporary directory, runs AST validation
// on the resulting Go files, and cleans up — all without touching the real
// workspace. Returns an error only for Go syntax failures; patch context
// mismatches are passed through silently (the real applyPatch step will
// surface them later).
func simulatePatch(diff string, touchedFiles []string) error {
	tmpDir, err := os.MkdirTemp("", "orch-sim-*")
	if err != nil {
		return nil // cannot create temp dir — skip simulation gracefully
	}
	defer os.RemoveAll(tmpDir)

	if err := copyFilesToDir(touchedFiles, tmpDir); err != nil {
		return nil // source files unreadable — skip simulation gracefully
	}

	if err := applyPatchInDir(diff, tmpDir); err != nil {
		// Context mismatch or malformed diff: the real applyPatch will catch this.
		return nil
	}

	return validateGoSyntaxInDir(touchedFiles, tmpDir)
}

// copyFilesToDir copies each file in paths into destDir, preserving relative
// path structure under destDir. Non-existent files are skipped silently.
func copyFilesToDir(paths []string, destDir string) error {
	for _, src := range paths {
		data, err := os.ReadFile(src)
		if err != nil {
			continue // new file added by the patch — nothing to copy
		}
		dst := filepath.Join(destDir, src)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// applyPatchInDir runs `patch -p1 -d dir` feeding diff on stdin.
func applyPatchInDir(diff, dir string) error {
	cmd := exec.Command("patch", "-p1", "-d", dir)
	cmd.Stdin = strings.NewReader(diff)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// validateGoSyntaxInDir parses each Go file from paths inside dir and returns
// an error listing any files that contain syntax errors.
func validateGoSyntaxInDir(paths []string, dir string) error {
	fset := token.NewFileSet()
	var failures []string
	for _, rel := range paths {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		candidate := filepath.Join(dir, rel)
		if _, err := os.Stat(candidate); err != nil {
			continue // file may have been deleted by the patch
		}
		if _, err := parser.ParseFile(fset, candidate, nil, parser.AllErrors); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", rel, err))
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("pre-write AST validation failed: %s", strings.Join(failures, "; "))
	}
	return nil
}

// validateSimulation is the validatePatch step that runs the simulation layer.
func validateSimulation(diff string, touchedFiles []string, task *Task) error {
	if err := simulatePatch(diff, touchedFiles); err != nil {
		logInfo("simulation_rejected", task.ID, err.Error())
		return err
	}
	return nil
}
