package audit

import (
	"encoding/json"
	"os"
)

// SaveFindings writes audit findings to disk as indented JSON.
func SaveFindings(path string, findings []Finding) error {
	data, err := json.MarshalIndent(findings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadFindings reads audit findings from an on-disk JSON file.
func LoadFindings(path string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var findings []Finding
	if err := json.Unmarshal(data, &findings); err != nil {
		return nil, err
	}
	return findings, nil
}
