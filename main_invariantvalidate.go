package main

import (
	"fmt"
	"strings"

	"github.com/opd-ai/orchestrator/audit"
)

// checkPostPatchInvariants validates touched Go source files against the
// architectural invariant registry after a patch has been applied.
// If violations are found the patch is reverted and an error is returned so
// the caller can mark the task blocked.
func checkPostPatchInvariants(diff string, touchedFiles []string, task *Task) error {
	reg, err := audit.LoadInvariantRegistry()
	if err != nil || reg == nil {
		return nil // invariant file absent — skip silently
	}

	var violations []audit.InvariantViolation
	for _, file := range touchedFiles {
		if !strings.HasSuffix(file, ".go") {
			continue
		}
		violations = append(violations, audit.CheckFileInvariants(file, reg)...)
	}
	if len(violations) == 0 {
		return nil
	}

	msgs := make([]string, 0, len(violations))
	for _, v := range violations {
		logInfo("invariant_violation", task.ID, v.String())
		msgs = append(msgs, v.String())
	}

	// Revert the patch so the workspace is clean before blocking the task.
	if rerr := revertPatch(diff); rerr != nil {
		logError("invariant_revert_failed", task.ID, rerr.Error())
	}
	return fmt.Errorf("post-patch invariant violations: %s", strings.Join(msgs, "; "))
}
