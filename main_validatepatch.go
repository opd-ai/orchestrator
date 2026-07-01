package main

import (
	"errors"
	"fmt"
	"strings"
)

func validatePatch(diff string, allowedFiles []string, task *Task) error {
	if err := validatePatchSize(diff, task); err != nil {
		return err
	}

	touchedFiles := filesTouched(diff)
	if err := validateTouchedFiles(touchedFiles, allowedFiles, task); err != nil {
		return err
	}

	return validateDeletionRatio(diff)
}

func deletionRatio(diff string) float64 {
	additions := 0
	deletions := 0

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "+"):
			additions++
		case strings.HasPrefix(line, "-"):
			deletions++
		}
	}

	total := additions + deletions
	if total == 0 {
		return 0
	}

	return float64(deletions) / float64(total)
}

func validatePatchSize(diff string, task *Task) error {
	limit := allowedPatchLines(task)
	if lineCount(diff) > limit {
		return fmt.Errorf("patch too large (limit=%d)", limit)
	}
	return nil
}

func validateTouchedFiles(touchedFiles, allowedFiles []string, task *Task) error {
	if len(touchedFiles) > maxFilesTouched {
		return errors.New("too many files modified")
	}
	if len(task.Files) == 0 {
		return nil
	}

	allowed := make(map[string]bool, len(allowedFiles))
	for _, file := range allowedFiles {
		allowed[file] = true
	}
	for _, file := range touchedFiles {
		if !allowed[file] {
			return fmt.Errorf("file %q is outside the allowed set", file)
		}
	}
	return nil
}

func validateDeletionRatio(diff string) error {
	if deletionRatio(diff) > 0.30 {
		return errors.New("patch deletes more than 30% of changed lines")
	}
	return nil
}
