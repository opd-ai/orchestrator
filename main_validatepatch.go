package main

import (
	"errors"
	"fmt"
)

func validatePatch(diff string, allowedFiles []string, task *Task) error {
	limit := allowedPatchLines(task)

	if lineCount(diff) > limit {
		return fmt.Errorf("patch too large (limit=%d)", limit)
	}

	if len(filesTouched(diff)) > maxFilesTouched {
		return errors.New("too many files modified")
	}

	return nil
}
