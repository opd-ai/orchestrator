package memory

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveRunUsesIsolatedWorktree(t *testing.T) {
	repo := initTestRepo(t)
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir(%q) error = %v", repo, err)
	}

	if err := os.WriteFile(filepath.Join(repo, "dirty.tmp"), []byte("local-only"), 0o644); err != nil {
		t.Fatalf("WriteFile(dirty.tmp) error = %v", err)
	}

	summary := RunSummary{Timestamp: time.Date(2026, 7, 2, 18, 0, 0, 0, time.UTC)}
	if err := SaveRun(summary); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	assertCurrentBranch(t, "main")
	assertFileExistsInBranch(t, "memories", filepath.Join(RunsDir, "2026-07-02T18-00-00.json"))
	assertFileMissingInBranch(t, "memories", "dirty.tmp")
}

func TestUpdateMetricsUsesIsolatedWorktree(t *testing.T) {
	repo := initTestRepo(t)
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("Chdir(%q) error = %v", repo, err)
	}

	if err := os.WriteFile(filepath.Join(repo, "dirty.tmp"), []byte("local-only"), 0o644); err != nil {
		t.Fatalf("WriteFile(dirty.tmp) error = %v", err)
	}

	if err := UpdateMetrics(RunSummary{LargestPatch: 10}); err != nil {
		t.Fatalf("UpdateMetrics() error = %v", err)
	}

	assertCurrentBranch(t, "main")
	assertFileExistsInBranch(t, "memories", MetricsFile)
	assertFileMissingInBranch(t, "memories", "dirty.tmp")
}

func initTestRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	runGit(t, repo, "init", "-b", "main")
	runGit(t, repo, "config", "user.email", "test@example.com")
	runGit(t, repo, "config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("tracked"), 0o644); err != nil {
		t.Fatalf("WriteFile(tracked.txt) error = %v", err)
	}
	runGit(t, repo, "add", "--", "tracked.txt")
	runGit(t, repo, "commit", "-m", "init")
	return repo
}

func assertCurrentBranch(t *testing.T, want string) {
	t.Helper()
	out := runGit(t, "", "rev-parse", "--abbrev-ref", "HEAD")
	if got := strings.TrimSpace(out); got != want {
		t.Fatalf("current branch = %q, want %q", got, want)
	}
}

func assertFileExistsInBranch(t *testing.T, branch, path string) {
	t.Helper()
	if _, err := exec.Command("git", "show", branch+":"+path).Output(); err != nil {
		t.Fatalf("expected %s to exist on %s: %v", path, branch, err)
	}
}

func assertFileMissingInBranch(t *testing.T, branch, path string) {
	t.Helper()
	if _, err := exec.Command("git", "show", branch+":"+path).Output(); err == nil {
		t.Fatalf("expected %s to be absent on %s", path, branch)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmdArgs := args
	if dir != "" {
		cmdArgs = append([]string{"-C", dir}, args...)
	}
	out, err := exec.Command("git", cmdArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return string(out)
}
