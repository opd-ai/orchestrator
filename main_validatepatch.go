package main

import (
	"errors"
	"fmt"
	"strings"
)

func validatePatch(diff string, allowedFiles []string, task *Task) error {
	limit := allowedPatchLines(task)
	touchedFiles := filesTouched(diff)

	if lineCount(diff) > limit {
		return fmt.Errorf("patch too large (limit=%d)", limit)
	}

	if len(touchedFiles) > maxFilesTouched {
		return errors.New("too many files modified")
	}

	if len(task.Files) > 0 {
		allowed := make(map[string]bool, len(allowedFiles))
		for _, file := range allowedFiles {
			allowed[file] = true
		}
		for _, file := range touchedFiles {
			if !allowed[file] {
				return fmt.Errorf("file %q is outside the allowed set", file)
			}
		}
	}

	if deletionRatio(diff) > 0.30 {
		return errors.New("patch deletes more than 30% of changed lines")
	}

	return nil
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
