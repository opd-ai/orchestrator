package memory

import (
	"os/exec"
)

func currentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return string(bytesTrim(out)), nil
}

func checkoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", name)
	return cmd.Run()
}

func ensureMemoryBranch() error {
	err := checkoutBranch(MemoryBranch)
	if err == nil {
		return nil
	}

	// create if missing
	cmd := exec.Command("git", "checkout", "--orphan", MemoryBranch)
	return cmd.Run()
}

func bytesTrim(b []byte) []byte {
	return []byte(string(b[:len(b)-1]))
}
