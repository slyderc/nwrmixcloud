// Package logger provides filename pattern validation for cross-platform compatibility.
// This ensures log filename patterns are safe to use on Windows, macOS, and Linux.
package logger

import (
	"fmt"
	"path/filepath"
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
// AIDEV-NOTE: Distinguishes between full paths and filename patterns
func ValidateFilenamePattern(pattern string) error {
	if pattern == "" {
		return nil // Empty pattern uses default, which is safe
	}

	// Determine if this is a full path or just a filename pattern
	isFullPath := isAbsolutePath(pattern)
	
	var filenameOnly string
	if isFullPath {
		// For full paths, extract just the filename part
		filenameOnly = extractFilename(pattern)
	} else {
		// For patterns, the whole thing should be treated as filename
		// Check if it contains path separators (which would be invalid)
		if strings.ContainsAny(pattern, "/\\") {
			return &FilenameValidationError{
				Pattern:      pattern,
				InvalidChars: []rune{'/', '\\'},
				Platform:     "all",
				Suggestion:   strings.ReplaceAll(strings.ReplaceAll(pattern, "/", "-"), "\\", "-"),
			}
		}
		filenameOnly = pattern
	}
	
	// Validate the filename part for platform-specific invalid characters
	invalidChars := findInvalidCharsInFilename(filenameOnly)
	if len(invalidChars) > 0 {
		suggestion := getSuggestionForFilename(pattern, filenameOnly, invalidChars)
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

// isAbsolutePath determines if a pattern represents a full path vs a filename pattern
func isAbsolutePath(pattern string) bool {
	// Unix absolute path
	if strings.HasPrefix(pattern, "/") {
		return true
	}
	// Windows absolute path (C:\, D:\, etc.)
	if len(pattern) >= 3 && pattern[1] == ':' && (pattern[2] == '\\' || pattern[2] == '/') {
		return true
	}
	// UNC path (\\server\share)
	if strings.HasPrefix(pattern, "\\\\") {
		return true
	}
	return false
}

// findInvalidCharsInFilename returns invalid characters found in the filename part only
// AIDEV-NOTE: No longer rejects path separators since they're handled at pattern level
func findInvalidCharsInFilename(filename string) []rune {
	var invalid []rune
	
	// Universal invalid characters (only null byte for filenames in full paths)
	filenameInvalid := []rune{'\x00'}
	
	// Windows-specific invalid characters in filenames
	windowsInvalid := []rune{'<', '>', ':', '"', '|', '?', '*'}
	
	// Check for null bytes
	for _, char := range filenameInvalid {
		if strings.ContainsRune(filename, char) {
			invalid = append(invalid, char)
		}
	}
	
	// Check Windows-specific chars if on Windows
	if runtime.GOOS == "windows" {
		for _, char := range windowsInvalid {
			if strings.ContainsRune(filename, char) {
				invalid = append(invalid, char)
			}
		}
	}
	
	return invalid
}

// getSuggestionForFilename provides a safe alternative pattern
// AIDEV-NOTE: Now preserves directory path and only fixes filename
func getSuggestionForFilename(fullPattern, filename string, invalidChars []rune) string {
	// Start with the original filename
	suggestion := filename
	
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
	
	// Combine fixed filename with original directory path
	dir := extractDirectory(fullPattern)
	if dir == "" {
		return suggestion // No directory part
	}
	return dir + string(filepath.Separator) + suggestion
}

// extractFilename extracts the filename from a pattern, handling both Unix and Windows paths
func extractFilename(pattern string) string {
	// Handle Windows paths by checking for backslashes
	if strings.Contains(pattern, "\\") {
		// Windows path - split on backslash
		parts := strings.Split(pattern, "\\")
		return parts[len(parts)-1]
	}
	// Unix path or simple filename - use standard filepath.Base
	return filepath.Base(pattern)
}

// extractDirectory extracts the directory from a pattern, handling both Unix and Windows paths  
func extractDirectory(pattern string) string {
	// Handle Windows paths by checking for backslashes
	if strings.Contains(pattern, "\\") {
		// Windows path - split on backslash
		parts := strings.Split(pattern, "\\")
		if len(parts) <= 1 {
			return ""
		}
		return strings.Join(parts[:len(parts)-1], "\\")
	}
	// Unix path - use standard filepath.Dir
	dir := filepath.Dir(pattern)
	if dir == "." {
		return ""
	}
	return dir
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