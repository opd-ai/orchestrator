package memory

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// withMemoryWorktree opens a temporary worktree for MemoryBranch without
// changing the caller's current branch/worktree.
func withMemoryWorktree(fn func(path string) error) error {
	dir, err := os.MkdirTemp("", "orchestrator-memory-worktree-*")
	if err != nil {
		return fmt.Errorf("create temp worktree: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := addMemoryWorktree(dir); err != nil {
		return err
	}
	defer exec.Command("git", "worktree", "remove", "--force", dir).Run()

	return fn(dir)
}

func addMemoryWorktree(dir string) error {
	if memoryBranchExists() {
		if err := exec.Command("git", "worktree", "add", dir, MemoryBranch).Run(); err != nil {
			return fmt.Errorf("add memory worktree: %w", err)
		}
		return nil
	}
	if err := exec.Command("git", "worktree", "add", "-b", MemoryBranch, dir, "HEAD").Run(); err != nil {
		return fmt.Errorf("create memory worktree: %w", err)
	}
	return nil
}

func memoryBranchExists() bool {
	err := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+MemoryBranch).Run()
	return err == nil
}

func commitWorktreeChanges(worktreePath, message string, allowNoChange bool, paths ...string) error {
	addArgs := []string{"-C", worktreePath, "add", "--"}
	addArgs = append(addArgs, paths...)
	if err := exec.Command("git", addArgs...).Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	if err := exec.Command("git", "-C", worktreePath, "commit", "-m", message).Run(); err != nil {
		var exitErr *exec.ExitError
		if allowNoChange && errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return nil
		}
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

// bytesTrim drops the trailing newline from command output when present.
func bytesTrim(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	return []byte(string(b[:len(b)-1]))
}
