package main

import (
	"errors"
	"fmt"
	"strings"
)

const fullRewriteChangeThreshold = 20

func validatePatch(diff string, allowedFiles []string, task *Task) error {
	touchedFiles := filesTouched(diff)
	steps := []func() error{
		func() error { return validatePatchSize(diff, task) },
		func() error { return validateTouchedFiles(touchedFiles, allowedFiles, task) },
		func() error { return validatePatchShape(diff, task) },
		func() error { return validateDeletionRatio(diff) },
	}
	return runValidationSteps(steps)
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

func validatePatchShape(diff string, task *Task) error {
	if hasRename(diff) {
		return errors.New("patch contains unexpected rename")
	}
	if hasFullFileRewrite(diff) {
		return errors.New("patch appears to rewrite a full file")
	}
	return validateLineDeltaCaps(diff, task)
}

func hasRename(diff string) bool {
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "rename from ") || strings.HasPrefix(line, "rename to ") {
			return true
		}
	}
	return false
}

func hasFullFileRewrite(diff string) bool {
	seenHunk := false
	contextLines := 0
	additions := 0
	deletions := 0

	flush := func() bool {
		return seenHunk &&
			contextLines == 0 &&
			additions > 0 &&
			deletions > 0 &&
			additions+deletions >= fullRewriteChangeThreshold
	}

	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			if flush() {
				return true
			}
			seenHunk = false
			contextLines = 0
			additions = 0
			deletions = 0
			continue
		}
		if strings.HasPrefix(line, "@@") {
			seenHunk = true
			continue
		}
		if !seenHunk || strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- ") {
			continue
		}

		switch {
		case strings.HasPrefix(line, " "):
			contextLines++
		case strings.HasPrefix(line, "+"):
			additions++
		case strings.HasPrefix(line, "-"):
			deletions++
		}
	}

	return flush()
}

func validateLineDeltaCaps(diff string, task *Task) error {
	limit := perFileLineDeltaCap(task)
	deltaByFile := make(map[string]int)
	currentFile := ""
	inHunk := false

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ b/"):
			currentFile = strings.TrimPrefix(line, "+++ b/")
			inHunk = false
		case strings.HasPrefix(line, "@@"):
			inHunk = currentFile != ""
		case !inHunk || strings.HasPrefix(line, "+++ ") || strings.HasPrefix(line, "--- "):
			continue
		case strings.HasPrefix(line, "+"), strings.HasPrefix(line, "-"):
			deltaByFile[currentFile]++
			if deltaByFile[currentFile] > limit {
				return fmt.Errorf("file %q exceeds line delta cap (%d)", currentFile, limit)
			}
		}
	}
	return nil
}

func perFileLineDeltaCap(task *Task) int {
	return max(1, allowedPatchLines(task)/2)
}

func runValidationSteps(steps []func() error) error {
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}
