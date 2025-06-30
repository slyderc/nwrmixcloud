// Package logger provides filename pattern validation for cross-platform compatibility.
// This ensures log filename patterns are safe to use on Windows, macOS, and Linux.
package logger

import (
	"fmt"
	"runtime"
	"strings"
)

// FilenameValidationError represents an error in filename pattern validation
type FilenameValidationError struct {
	Pattern      string
	InvalidChars []rune
	Platform     string
	Suggestion   string
}

func (e *FilenameValidationError) Error() string {
	charList := make([]string, len(e.InvalidChars))
	for i, char := range e.InvalidChars {
		charList[i] = fmt.Sprintf("'%c'", char)
	}
	
	msg := fmt.Sprintf("invalid filename pattern %q contains invalid characters: %s", 
		e.Pattern, strings.Join(charList, ", "))
	
	if e.Platform != "all" {
		msg += fmt.Sprintf(" (invalid on %s)", e.Platform)
	}
	
	if e.Suggestion != "" {
		msg += fmt.Sprintf(". Suggestion: %s", e.Suggestion)
	}
	
	return msg
}

// ValidateFilenamePattern validates that a filename pattern is safe for the current platform
func ValidateFilenamePattern(pattern string) error {
	if pattern == "" {
		return nil // Empty pattern uses default, which is safe
	}

	// AIDEV-NOTE: Check for platform-specific invalid characters
	invalidChars := findInvalidChars(pattern)
	if len(invalidChars) > 0 {
		suggestion := getSuggestion(pattern, invalidChars)
		platform := "all"
		if runtime.GOOS == "windows" {
			platform = "Windows"
		}
		
		return &FilenameValidationError{
			Pattern:      pattern,
			InvalidChars: invalidChars,
			Platform:     platform,
			Suggestion:   suggestion,
		}
	}

	return nil
}

// findInvalidChars returns invalid characters found in the pattern
func findInvalidChars(pattern string) []rune {
	var invalid []rune
	
	// Universal invalid characters (path separators)
	universalInvalid := []rune{'/', '\\', '\x00'}
	
	// Windows-specific invalid characters
	windowsInvalid := []rune{'<', '>', ':', '"', '|', '?', '*'}
	
	// Check universal invalid chars
	for _, char := range universalInvalid {
		if strings.ContainsRune(pattern, char) {
			invalid = append(invalid, char)
		}
	}
	
	// Check Windows-specific chars if on Windows
	if runtime.GOOS == "windows" {
		for _, char := range windowsInvalid {
			if strings.ContainsRune(pattern, char) {
				invalid = append(invalid, char)
			}
		}
	}
	
	return invalid
}

// getSuggestion provides a safe alternative pattern
func getSuggestion(pattern string, invalidChars []rune) string {
	suggestion := pattern
	
	// AIDEV-NOTE: Replace common problematic patterns with safe alternatives
	replacements := map[rune]string{
		'/':  "-",  // MM/DD/YYYY -> MM-DD-YYYY
		'\\': "-",  // Similar for backslash
		':':  "-",  // HH:MM:SS -> HH-MM-SS
		'|':  "-",  // YYYY|MM -> YYYY-MM
		'*':  "X",  // app-* -> app-X
		'?':  "X",  // app-? -> app-X
		'<':  "",   // Remove
		'>':  "",   // Remove
		'"':  "",   // Remove
	}
	
	for _, char := range invalidChars {
		if replacement, exists := replacements[char]; exists {
			suggestion = strings.ReplaceAll(suggestion, string(char), replacement)
		}
	}
	
	// Clean up multiple consecutive dashes
	for strings.Contains(suggestion, "--") {
		suggestion = strings.ReplaceAll(suggestion, "--", "-")
	}
	
	return suggestion
}

// GetSafeFilenamePatterns returns a list of recommended safe patterns
func GetSafeFilenamePatterns() []string {
	return []string{
		"app-YYYYMMDD.log",           // Compact format
		"app-YYYY-MM-DD.log",         // ISO format with dashes
		"app-YYYY.MM.DD.log",         // Dot-separated format
		"app_YYYY_MM_DD.log",         // Underscore format
		"app-YYYYMMDD-HHMMSS.log",    // With time (compact)
		"app-YYYY-MM-DD-HH-MM.log",   // With time (readable)
	}
}

// GetUnsafeFilenamePatterns returns examples of patterns to avoid
func GetUnsafeFilenamePatterns() map[string]string {
	return map[string]string{
		"app-MM/DD/YYYY.log":    "Forward slashes create subdirectories",
		"app-HH:MM:SS.log":      "Colons invalid on Windows",
		"app-YYYY|MM|DD.log":    "Pipes invalid on Windows", 
		"app-*-YYYYMMDD.log":    "Asterisks invalid on Windows",
		"app-?-YYYYMMDD.log":    "Question marks invalid on Windows",
		"app-<YYYY>.log":        "Angle brackets invalid on Windows",
		"app-\"YYYY\".log":      "Quotes invalid on Windows",
		"app\\YYYY\\MM.log":     "Backslashes create subdirectories",
	}
}