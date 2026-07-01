package main

import (
	"errors"
	"strings"
)

// ChangeType classifies the structural transformation a task should apply.
// Using explicit types makes prompts more deterministic and enables schema validation.
type ChangeType string

const (
	ChangeTypeInsertFunction ChangeType = "INSERT_FUNCTION"
	ChangeTypeModifyFunction ChangeType = "MODIFY_FUNCTION"
	ChangeTypeAddImport      ChangeType = "ADD_IMPORT"
	ChangeTypeModifyStruct   ChangeType = "MODIFY_STRUCT"
	ChangeTypeGeneral        ChangeType = "GENERAL"
)

// validateDSLSchema checks that diff structurally matches the expected changeType.
// Returns nil for ChangeTypeGeneral, empty, or unrecognised change types.
func validateDSLSchema(diff string, changeType ChangeType) error {
	switch changeType {
	case ChangeTypeInsertFunction:
		return requiresDiffPattern(diff, "func ",
			"INSERT_FUNCTION requires a new function definition in the diff (+func ...)")
	case ChangeTypeAddImport:
		return requiresDiffPattern(diff, "\"",
			"ADD_IMPORT requires an import string literal in the diff (+\"...\")")
	case ChangeTypeModifyFunction, ChangeTypeModifyStruct, ChangeTypeGeneral, "":
		return nil
	default:
		return nil
	}
}

// requiresDiffPattern returns an error unless any added line (+) in diff contains pattern.
func requiresDiffPattern(diff, pattern, msg string) error {
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") &&
			!strings.HasPrefix(line, "+++") &&
			strings.Contains(line, pattern) {
			return nil
		}
	}
	return errors.New(msg)
}
