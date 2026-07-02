package memory

import (
	"os/exec"
)

// currentBranch returns the current git branch name.
func currentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return string(bytesTrim(out)), nil
}

// checkoutBranch switches the repository to the named git branch.
func checkoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", name)
	return cmd.Run()
}

// ensureMemoryBranch checks out the memory branch or creates it as an orphan branch.
func ensureMemoryBranch() error {
	err := checkoutBranch(MemoryBranch)
	if err == nil {
		return nil
	}

	// create if missing
	cmd := exec.Command("git", "checkout", "--orphan", MemoryBranch)
	return cmd.Run()
}

// bytesTrim drops the trailing newline from command output when present.
func bytesTrim(b []byte) []byte {
	if len(b) == 0 {
		return b
	}
	return []byte(string(b[:len(b)-1]))
}
