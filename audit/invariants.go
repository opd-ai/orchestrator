package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const InvariantRegistryPath = "architecture/invariants.json"

// Invariant represents a single architectural constraint.
type Invariant struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	MaxValue    int    `json:"max_value,omitempty"`
}

// InvariantRegistry holds the project's architectural constraints.
type InvariantRegistry struct {
	Invariants []Invariant `json:"invariants"`
}

// LoadInvariantRegistry reads InvariantRegistryPath if it exists.
// Returns nil and a non-nil error when the file is absent or malformed.
func LoadInvariantRegistry() (*InvariantRegistry, error) {
	data, err := os.ReadFile(InvariantRegistryPath)
	if err != nil {
		return nil, err
	}
	var reg InvariantRegistry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("invariants parse error: %w", err)
	}
	return &reg, nil
}

// Summary returns a compact multi-line string listing each invariant,
// suitable for injection into a planner prompt.
func (r *InvariantRegistry) Summary() string {
	if r == nil || len(r.Invariants) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("ARCHITECTURAL INVARIANTS:\n")
	for _, inv := range r.Invariants {
		b.WriteString("- " + inv.Description + "\n")
	}
	return strings.TrimSpace(b.String())
}
