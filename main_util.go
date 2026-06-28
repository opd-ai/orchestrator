package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func currentGitBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func averageRetries(totalRetries, tasks int) float64 {
	if tasks == 0 {
		return 0
	}
	return float64(totalRetries) / float64(tasks)
}

func exitOnErr(err error) {
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}
