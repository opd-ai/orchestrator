package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const fullRewriteChangeThreshold = 20

func validatePatch(diff string, allowedFiles []string, task *Task) error {
	confidence := evaluatePatchConfidence(diff)
	logInfo("patch_confidence", task.ID, confidence.message())

	touchedFiles := filesTouched(diff)
	steps := []func() error{
		func() error { return validateTransformOnly(task) },
		func() error { return validatePatchRisk(diff, task) },
		func() error { return validatePatchSize(diff, task) },
		func() error { return validateTouchedFiles(touchedFiles, allowedFiles, task) },
		func() error { return validatePatchShape(diff, task) },
		func() error { return validateDeletionRatio(diff) },
		func() error { return validateDSLSchema(diff, task.ChangeType) },
		func() error { return validateSimulation(diff, touchedFiles, task) },
	}
	return runValidationSteps(steps)
}

// validateTransformOnly rejects tasks that lack a ChangeType when --transform-only is set.
func validateTransformOnly(task *Task) error {
	if transformOnly && task.ChangeType == "" {
		return errors.New("transform-only mode requires task.change_type to be set")
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

func validatePatchSize(diff string, task *Task) error {
	limit := allowedPatchLines(task)
	if changedLineCount(diff) > limit {
		return fmt.Errorf("patch too large (limit=%d)", limit)
	}
	return nil
}

// changedLineCount counts only '+' and '-' lines in a diff, excluding file
// header lines (+++/---). This matches the semantics used by deletionRatio and
// correctly measures the actual edit size rather than inflating the count with
// diff header overhead (F-15).
func changedLineCount(diff string) int {
	count := 0
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			continue
		case strings.HasPrefix(line, "+"), strings.HasPrefix(line, "-"):
			count++
		}
	}
	return count
}

func validateTouchedFiles(touchedFiles, allowedFiles []string, task *Task) error {
	if len(touchedFiles) > maxFilesTouched+fileCapBonus() {
		return errors.New("too many files modified")
	}
	if err := validateTouchedFilePaths(touchedFiles); err != nil {
		return err
	}
	if len(task.Files) == 0 {
		return nil
	}
	return validateAllowedTouchedFiles(touchedFiles, allowedFiles)
}

func validateTouchedFilePaths(touchedFiles []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve repository root: %w", err)
	}
	for _, file := range touchedFiles {
		if err := validateTouchedFilePath(file, wd); err != nil {
			return err
		}
	}
	return nil
}

func validateAllowedTouchedFiles(touchedFiles, allowedFiles []string) error {
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

func validateTouchedFilePath(file, wd string) error {
	if strings.TrimSpace(file) == "" {
		return errors.New("invalid touched file path")
	}
	if filepath.IsAbs(file) {
		return fmt.Errorf("file %q escapes repository root", file)
	}

	clean := filepath.Clean(file)
	candidate := filepath.Join(wd, clean)
	resolvedRoot, err := filepath.EvalSymlinks(wd)
	if err != nil {
		return fmt.Errorf("resolve repository root symlinks: %w", err)
	}
	resolvedCandidate, err := resolvePathForContainment(candidate)
	if err != nil {
		return fmt.Errorf("validate path %q: %w", file, err)
	}

	rel, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil {
		return fmt.Errorf("validate path %q: %w", file, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("file %q escapes repository root", file)
	}
	return nil
}

func resolvePathForContainment(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if !os.IsNotExist(err) {
		return "", err
	}
	return resolvePathFromExistingParent(path)
}

func resolvePathFromExistingParent(path string) (string, error) {
	candidate := path
	for {
		parent := filepath.Dir(candidate)
		rel, relErr := filepath.Rel(parent, path)
		if relErr != nil {
			return "", relErr
		}
		resolvedParent, err := filepath.EvalSymlinks(parent)
		if err == nil {
			return filepath.Join(resolvedParent, rel), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		if parent == candidate {
			return "", fmt.Errorf("no existing parent found for %q", path)
		}
		candidate = parent
	}
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

// perFileLineDeltaCap returns the maximum number of changed lines allowed for a
// single file within a patch. For single-file patches the full budget applies;
// for multi-file patches half the budget is allocated per file to leave room
// for sibling files (F-18).
func perFileLineDeltaCap(task *Task) int {
	if len(task.Files) <= 1 {
		return max(1, allowedPatchLines(task))
	}
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
