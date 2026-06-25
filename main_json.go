package main

import (
	"encoding/json"
	"errors"
	"strings"
)

func extractJSON(content string) (string, error) {
	content = strings.TrimSpace(content)

	// Remove ```json or ``` fences
	if strings.HasPrefix(content, "```") {
		parts := strings.Split(content, "```")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "{") || strings.HasPrefix(p, "[") {
				content = p
				break
			}
		}
	}

	// Try to find first JSON object or array manually
	start := strings.IndexAny(content, "[{")
	if start == -1 {
		return "", errors.New("no JSON found in response")
	}

	content = content[start:]

	// Try progressive trimming from end until valid JSON
	for i := len(content); i > 0; i-- {
		candidate := content[:i]
		var js interface{}
		if json.Unmarshal([]byte(candidate), &js) == nil {
			return candidate, nil
		}
	}

	return "", errors.New("could not extract valid JSON")
}
