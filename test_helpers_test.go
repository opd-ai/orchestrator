package main

import (
	"os/exec"
	"testing"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	runCmd(t, "git", "init", dir)
	runCmd(t, "git", "-C", dir, "config", "user.email", "test@example.com")
	runCmd(t, "git", "-C", dir, "config", "user.name", "Test User")
}

func runCmd(t *testing.T, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, string(out))
	}
	return string(out)
}
