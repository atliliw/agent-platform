// Package parser provides document parsing functionality
package parser

import (
	"bytes"
	"fmt"
	"strings"
)

// Parse parses a file and returns its text content
func Parse(filename string, content []byte) (string, error) {
	// Get file extension
	ext := strings.ToLower(filename)
	if idx := strings.LastIndex(ext, "."); idx >= 0 {
		ext = ext[idx+1:]
	}

	switch ext {
	case "txt", "md":
		return string(content), nil

	case "json":
		return parseJSON(content)

	case "csv":
		return parseCSV(content)

	case "pdf":
		return parsePDF(content)

	case "docx":
		return parseDocx(content)

	default:
		// Try as plain text
		return string(content), nil
	}
}

func parseJSON(content []byte) (string, error) {
	// Simple JSON to text extraction
	// Remove brackets and quotes, extract values
	var buf bytes.Buffer
	inString := false
	escape := false

	for _, b := range content {
		switch b {
		case '"':
			if !escape {
				inString = !inString
			}
			escape = false
		case '\\':
			escape = true
		case '{', '}', '[', ']', ',':
			if inString {
				buf.WriteByte(b)
			} else if b == ',' || b == '{' || b == '[' {
				buf.WriteByte(' ')
			}
		case ':':
			if inString {
				buf.WriteByte(b)
			} else {
				buf.WriteByte(' ')
			}
		default:
			buf.WriteByte(b)
			escape = false
		}
	}

	return strings.TrimSpace(buf.String()), nil
}

func parseCSV(content []byte) (string, error) {
	lines := strings.Split(string(content), "\n")
	var texts []string

	for i, line := range lines {
		if i == 0 {
			continue // Skip header
		}
		line = strings.TrimSpace(line)
		if line != "" {
			texts = append(texts, line)
		}
	}

	return strings.Join(texts, "\n"), nil
}

func parsePDF(content []byte) (string, error) {
	// Simple PDF text extraction
	// In production, use a proper PDF library
	return "", fmt.Errorf("PDF parsing requires external library")
}

func parseDocx(content []byte) (string, error) {
	// Simple DOCX text extraction
	// In production, use a proper DOCX library
	return "", fmt.Errorf("DOCX parsing requires external library")
}