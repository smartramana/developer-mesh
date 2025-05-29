package parsers

import (
	"strings"
)

// countLines counts the number of lines in a string
func countLines(s string) int {
	return strings.Count(s, "\n") + 1
}

// countLinesUpTo counts the number of lines in a string up to a given position
func countLinesUpTo(s string, pos int) int {
	if pos > len(s) {
		pos = len(s)
	}
	return strings.Count(s[:pos], "\n")
}

// getLineNumberFromPos gets the line number from a position in the code
func getLineNumberFromPos(code string, pos int) int {
	if pos > len(code) {
		pos = len(code)
	}
	return strings.Count(code[:pos], "\n")
}

// containsString checks if a string slice contains a specific string
func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
