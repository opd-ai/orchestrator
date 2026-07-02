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

	// Find the outermost JSON object or array using forward bracket matching.
	start := strings.IndexAny(content, "[{")
	if start == -1 {
		return "", errors.New("no JSON found in response")
	}

	open := rune(content[start])
	var close rune
	if open == '[' {
		close = ']'
	} else {
		close = '}'
	}

	depth := 0
	inString := false
	escaped := false
	for i, r := range content[start:] {
		switch {
		case escaped:
			escaped = false
		case inString && r == '\\':
			escaped = true
		case r == '"':
			inString = !inString
		case inString:
			// skip
		case r == open:
			depth++
		case r == close:
			depth--
			if depth == 0 {
				candidate := content[start : start+i+1]
				var js interface{}
				if json.Unmarshal([]byte(candidate), &js) == nil {
					return candidate, nil
				}
				return "", errors.New("could not extract valid JSON")
			}
		}
	}

	return "", errors.New("could not extract valid JSON")
}
