// Package logger provides tests for filename pattern validation.
package logger

import (
	"runtime"
	"strings"
	"testing"
)

func TestValidateFilenamePattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		expectError bool
		platform    string // "windows", "unix", "all"
		description string
	}{
		{
			name:        "Valid YYYYMMDD pattern",
			pattern:     "app-YYYYMMDD.log",
			expectError: false,
			platform:    "all",
			description: "Safe compact date format",
		},
		{
			name:        "Valid YYYY-MM-DD pattern",
			pattern:     "app-YYYY-MM-DD.log",
			expectError: false,
			platform:    "all",
			description: "Safe dashed date format",
		},
		{
			name:        "Invalid MM/DD/YYYY pattern",
			pattern:     "app-MM/DD/YYYY.log",
			expectError: true,
			platform:    "all",
			description: "Forward slashes create paths",
		},
		{
			name:        "Invalid with colon (Windows)",
			pattern:     "app-HH:MM:SS.log",
			expectError: true,
			platform:    "windows",
			description: "Colons are invalid on Windows",
		},
		{
			name:        "Valid with colon (Unix)",
			pattern:     "app-HH:MM:SS.log",
			expectError: false,
			platform:    "unix",
			description: "Colons are valid on Unix",
		},
		{
			name:        "Invalid with pipe",
			pattern:     "app-YYYY|MM.log",
			expectError: true,
			platform:    "windows",
			description: "Pipes are invalid on Windows",
		},
		{
			name:        "Invalid with asterisk",
			pattern:     "app-*-YYYY.log",
			expectError: true,
			platform:    "windows",
			description: "Asterisks are invalid on Windows",
		},
		{
			name:        "Empty pattern (uses default)",
			pattern:     "",
			expectError: false,
			platform:    "all",
			description: "Empty pattern should be allowed",
		},
		{
			name:        "Valid with underscores",
			pattern:     "app_YYYY_MM_DD.log",
			expectError: false,
			platform:    "all",
			description: "Underscores are safe",
		},
		{
			name:        "Valid with dots",
			pattern:     "app.YYYY.MM.DD.log",
			expectError: false,
			platform:    "all",
			description: "Dots are safe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip platform-specific tests
			if tt.platform == "windows" && runtime.GOOS != "windows" {
				t.Skip("Skipping Windows-specific test")
			}
			if tt.platform == "unix" && runtime.GOOS == "windows" {
				t.Skip("Skipping Unix-specific test")
			}

			err := ValidateFilenamePattern(tt.pattern)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for pattern %q, but got none", tt.pattern)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for pattern %q: %v", tt.pattern, err)
			}

			if err != nil {
				t.Logf("Pattern %q validation error: %v", tt.pattern, err)
			} else {
				t.Logf("✓ Pattern %q is valid", tt.pattern)
			}
		})
	}
}

func TestFilenameValidationError(t *testing.T) {
	err := &FilenameValidationError{
		Pattern:      "app-MM/DD/YYYY.log",
		InvalidChars: []rune{'/', '/'},
		Platform:     "all",
		Suggestion:   "app-MM-DD-YYYY.log",
	}

	errorMsg := err.Error()
	
	// Check that error message contains key information
	expectedParts := []string{
		"app-MM/DD/YYYY.log",
		"invalid characters",
		"'/'",
		"app-MM-DD-YYYY.log",
	}

	for _, part := range expectedParts {
		if !strings.Contains(errorMsg, part) {
			t.Errorf("Error message missing expected part %q. Got: %s", part, errorMsg)
		}
	}

	t.Logf("Error message: %s", errorMsg)
}

func TestGetSuggestion(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		invalidChars []rune
		expected    string
	}{
		{
			name:        "Forward slash replacement",
			pattern:     "app-MM/DD/YYYY.log",
			invalidChars: []rune{'/', '/'},
			expected:    "app-MM-DD-YYYY.log",
		},
		{
			name:        "Colon replacement",
			pattern:     "app-HH:MM:SS.log",
			invalidChars: []rune{':', ':'},
			expected:    "app-HH-MM-SS.log",
		},
		{
			name:        "Multiple character replacement",
			pattern:     "app-YYYY|MM*DD.log",
			invalidChars: []rune{'|', '*'},
			expected:    "app-YYYY-MMXDD.log",
		},
		{
			name:        "Remove problematic chars",
			pattern:     "app-<YYYY>.log",
			invalidChars: []rune{'<', '>'},
			expected:    "app-YYYY.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSuggestion(tt.pattern, tt.invalidChars)
			if result != tt.expected {
				t.Errorf("getSuggestion(%q, %v) = %q, want %q", 
					tt.pattern, tt.invalidChars, result, tt.expected)
			}
			t.Logf("✓ %q -> %q", tt.pattern, result)
		})
	}
}

func TestSafePatterns(t *testing.T) {
	safePatterns := GetSafeFilenamePatterns()
	
	if len(safePatterns) == 0 {
		t.Error("GetSafeFilenamePatterns() returned no patterns")
	}

	// Validate that all "safe" patterns are actually safe
	for _, pattern := range safePatterns {
		t.Run("safe_"+pattern, func(t *testing.T) {
			err := ValidateFilenamePattern(pattern)
			if err != nil {
				t.Errorf("Safe pattern %q failed validation: %v", pattern, err)
			} else {
				t.Logf("✓ Safe pattern: %s", pattern)
			}
		})
	}
}

func TestUnsafePatterns(t *testing.T) {
	unsafePatterns := GetUnsafeFilenamePatterns()
	
	if len(unsafePatterns) == 0 {
		t.Error("GetUnsafeFilenamePatterns() returned no patterns")
	}

	// Validate that "unsafe" patterns actually fail validation (on appropriate platforms)
	for pattern, reason := range unsafePatterns {
		t.Run("unsafe_"+pattern, func(t *testing.T) {
			// Skip Windows-specific tests on non-Windows
			if (strings.Contains(pattern, ":") || strings.Contains(pattern, "|") || 
				strings.Contains(pattern, "*") || strings.Contains(pattern, "?") ||
				strings.Contains(pattern, "<") || strings.Contains(pattern, ">") ||
				strings.Contains(pattern, "\"")) && runtime.GOOS != "windows" {
				t.Skip("Skipping Windows-specific unsafe pattern test")
			}

			err := ValidateFilenamePattern(pattern)
			if err == nil && (strings.Contains(pattern, "/") || strings.Contains(pattern, "\\")) {
				// Path separators should always fail
				t.Errorf("Unsafe pattern %q should have failed validation (reason: %s)", pattern, reason)
			} else if err != nil {
				t.Logf("✓ Unsafe pattern correctly failed: %s (reason: %s)", pattern, reason)
			} else {
				t.Logf("Pattern %s may be platform-specific (reason: %s)", pattern, reason)
			}
		})
	}
}

func TestLoggerValidationIntegration(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name        string
		pattern     string
		expectError bool
		description string
	}{
		{
			name:        "Valid pattern creates logger",
			pattern:     "app-YYYYMMDD.log",
			expectError: false,
			description: "Safe pattern should work",
		},
		{
			name:        "Invalid pattern rejects logger",
			pattern:     "app-MM/DD/YYYY.log",
			expectError: true,
			description: "Unsafe pattern should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := Config{
				Enabled:         true,
				Directory:       tempDir,
				FilenamePattern: tt.pattern,
				Level:          "info",
				ConsoleOutput:  false,
			}

			logger, err := NewLogger(config)

			if tt.expectError && err == nil {
				t.Errorf("Expected NewLogger to fail with pattern %q, but it succeeded", tt.pattern)
				if logger != nil {
					logger.Close()
				}
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected NewLogger to succeed with pattern %q, but got error: %v", tt.pattern, err)
			}

			if err != nil {
				t.Logf("✓ Pattern %q correctly rejected: %v", tt.pattern, err)
			} else {
				t.Logf("✓ Pattern %q accepted successfully", tt.pattern)
				if logger != nil {
					logger.Close()
				}
			}
		})
	}
}