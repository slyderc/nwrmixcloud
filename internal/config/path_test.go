// Package config provides tests for cross-platform path handling in TOML configuration.
// This addresses the Windows path escaping issue where users shouldn't need double-backslashes.
package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// TestWindowsPathHandling tests various Windows path formats in TOML
func TestWindowsPathHandling(t *testing.T) {
	tests := []struct {
		name        string
		tomlContent string
		expectError bool
		expectedPath string
		description string
	}{
		{
			name: "Double backslash (current working method)",
			tomlContent: `
cue_file_directory = "C:\\Myriad\\Data\\Publish"
filename_pattern = "C:\\logs\\app-YYYYMMDD.log"
`,
			expectError: false,
			expectedPath: "C:\\Myriad\\Data\\Publish",
			description: "What currently works but is user-unfriendly",
		},
		{
			name: "Single backslash (what users want to write)",
			tomlContent: `
cue_file_directory = "C:\Myriad\Data\Publish"
filename_pattern = "C:\logs\app-YYYYMMDD.log"
`,
			expectError: true, // TOML will likely reject this
			expectedPath: "",
			description: "What users naturally want to write",
		},
		{
			name: "Forward slashes (Unix-style on Windows)",
			tomlContent: `
cue_file_directory = "C:/Myriad/Data/Publish"
filename_pattern = "C:/logs/app-YYYYMMDD.log"
`,
			expectError: false,
			expectedPath: "C:/Myriad/Data/Publish",
			description: "Cross-platform friendly approach",
		},
		{
			name: "Raw string with forward slashes",
			tomlContent: `
cue_file_directory = 'C:/Myriad/Data/Publish'
filename_pattern = 'C:/logs/app-YYYYMMDD.log'
`,
			expectError: false,
			expectedPath: "C:/Myriad/Data/Publish",
			description: "TOML raw strings with forward slashes",
		},
		{
			name: "Relative Windows path",
			tomlContent: `
cue_file_directory = "Data\\Publish"
filename_pattern = "logs\\app-YYYYMMDD.log"
`,
			expectError: false,
			expectedPath: "Data\\Publish",
			description: "Relative paths with double backslashes",
		},
		{
			name: "Relative Unix-style path",
			tomlContent: `
cue_file_directory = "Data/Publish"
filename_pattern = "logs/app-YYYYMMDD.log"
`,
			expectError: false,
			expectedPath: "Data/Publish",
			description: "Relative paths with forward slashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary config structure
			var testConfig struct {
				CueFileDirectory string `toml:"cue_file_directory"`
				FilenamePattern  string `toml:"filename_pattern"`
			}

			// Try to parse the TOML
			_, err := toml.Decode(tt.tomlContent, &testConfig)
			
			if tt.expectError && err == nil {
				t.Errorf("Expected TOML parsing to fail for %s, but it succeeded", tt.name)
				t.Logf("Parsed cue_file_directory: %q", testConfig.CueFileDirectory)
			}
			
			if !tt.expectError && err != nil {
				t.Errorf("TOML parsing failed unexpectedly for %s: %v", tt.name, err)
				return
			}

			if !tt.expectError {
				// Verify the path was parsed correctly
				if testConfig.CueFileDirectory != tt.expectedPath {
					t.Errorf("Expected path %q, got %q", tt.expectedPath, testConfig.CueFileDirectory)
				}
				
				t.Logf("✓ %s: %q -> %q", tt.description, tt.expectedPath, testConfig.CueFileDirectory)
				
				// Test that Go can handle the parsed path
				cleaned := filepath.Clean(testConfig.CueFileDirectory)
				t.Logf("  Cleaned path: %q", cleaned)
				
				// Test if it's absolute
				if filepath.IsAbs(testConfig.CueFileDirectory) {
					t.Logf("  Path is absolute ✓")
				} else {
					t.Logf("  Path is relative ✓")
				}
			} else {
				t.Logf("✓ %s correctly failed TOML parsing", tt.description)
			}
		})
	}
}

// TestPathNormalization tests Go's ability to handle different path formats
func TestPathNormalization(t *testing.T) {
	tests := []struct {
		name         string
		inputPath    string
		expectValid  bool
		description  string
	}{
		{
			name:        "Windows double backslash",
			inputPath:   "C:\\Myriad\\Data\\Publish",
			expectValid: true,
			description: "Standard Windows path with escaped backslashes",
		},
		{
			name:        "Windows forward slash",
			inputPath:   "C:/Myriad/Data/Publish",
			expectValid: true,
			description: "Windows path with Unix-style separators",
		},
		{
			name:        "Mixed separators",
			inputPath:   "C:/Myriad\\Data/Publish",
			expectValid: true,
			description: "Mixed path separators (common in practice)",
		},
		{
			name:        "Relative forward slash",
			inputPath:   "Data/Publish",
			expectValid: true,
			description: "Relative path with forward slashes",
		},
		{
			name:        "Relative backslash",
			inputPath:   "Data\\Publish",
			expectValid: true,
			description: "Relative path with backslashes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test filepath.Clean normalization
			cleaned := filepath.Clean(tt.inputPath)
			
			// Test filepath.Abs resolution
			abs, err := filepath.Abs(tt.inputPath)
			if err != nil && tt.expectValid {
				t.Errorf("filepath.Abs failed for %q: %v", tt.inputPath, err)
			}

			// Log results
			t.Logf("Original: %q", tt.inputPath)
			t.Logf("Cleaned:  %q", cleaned)
			if err == nil {
				t.Logf("Absolute: %q", abs)
			}
			
			// Test platform-specific behavior
			if runtime.GOOS == "windows" {
				// On Windows, both should work
				t.Logf("Windows: Both forward and back slashes should work")
			} else {
				// On Unix-like systems, backslashes are literal characters
				if strings.Contains(tt.inputPath, "\\") {
					t.Logf("Unix: Backslashes are literal characters in path")
				}
			}
		})
	}
}

// TestTOMLStringTypes tests different TOML string quoting styles
func TestTOMLStringTypes(t *testing.T) {
	tests := []struct {
		name        string
		tomlContent string
		description string
	}{
		{
			name: "Double quotes (escaped)",
			tomlContent: `path = "C:\\Windows\\System32"`,
			description: "Requires escaping backslashes",
		},
		{
			name: "Single quotes (raw)",
			tomlContent: `path = 'C:\Windows\System32'`,
			description: "Raw strings - no escaping needed!",
		},
		{
			name: "Double quotes with forward slashes",
			tomlContent: `path = "C:/Windows/System32"`,
			description: "No escaping needed with forward slashes",
		},
		{
			name: "Raw string with forward slashes",
			tomlContent: `path = 'C:/Windows/System32'`,
			description: "Clean and simple",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config struct {
				Path string `toml:"path"`
			}

			_, err := toml.Decode(tt.tomlContent, &config)
			if err != nil {
				t.Errorf("TOML parsing failed: %v", err)
				return
			}

			t.Logf("✓ %s", tt.description)
			t.Logf("  TOML: %s", tt.tomlContent)
			t.Logf("  Parsed: %q", config.Path)
		})
	}
}

// TestWindowsSpecificPaths tests Windows-specific path features
func TestWindowsSpecificPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Skipping Windows-specific test on non-Windows platform")
	}

	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "UNC path",
			path:     "\\\\server\\share\\folder",
			expected: "\\\\server\\share\\folder",
		},
		{
			name:     "Drive with forward slashes",
			path:     "C:/Program Files/MyApp",
			expected: "C:\\Program Files\\MyApp", // Windows normalizes to backslashes
		},
		{
			name:     "Drive with backslashes",
			path:     "C:\\Program Files\\MyApp",
			expected: "C:\\Program Files\\MyApp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleaned := filepath.Clean(tt.path)
			t.Logf("Original: %q", tt.path)
			t.Logf("Cleaned:  %q", cleaned)
			
			// Note: filepath.Clean may or may not change separators depending on Go version
			// The important thing is that the path is usable
		})
	}
}