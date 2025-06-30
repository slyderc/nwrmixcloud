package dateutil

import (
	"testing"
	"time"
)

func TestFormatDateToGoLayout(t *testing.T) {
	tests := []struct {
		name       string
		userFormat string
		expected   string
	}{
		{
			name:       "M/D/YYYY format",
			userFormat: "M/D/YYYY",
			expected:   "1/2/2006",
		},
		{
			name:       "MM/DD/YYYY format",
			userFormat: "MM/DD/YYYY",
			expected:   "01/02/2006",
		},
		{
			name:       "D-M-YY format",
			userFormat: "D-M-YY",
			expected:   "2-1-06",
		},
		{
			name:       "YYYY.MM.DD format",
			userFormat: "YYYY.MM.DD",
			expected:   "2006.01.02",
		},
		{
			name:       "YYYYMMDD format (log filename style)",
			userFormat: "YYYYMMDD",
			expected:   "20060102",
		},
		{
			name:       "Mixed separators",
			userFormat: "M/DD-YYYY",
			expected:   "1/02-2006",
		},
		{
			name:       "Empty string",
			userFormat: "",
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDateToGoLayout(tt.userFormat)
			if result != tt.expected {
				t.Errorf("FormatDateToGoLayout(%q) = %q, want %q",
					tt.userFormat, result, tt.expected)
			}
		})
	}
}

func TestFormatDateWithPattern(t *testing.T) {
	testTime := time.Date(2025, 6, 28, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		userPattern string
		expected    string
	}{
		{
			name:        "M/D/YYYY format",
			userPattern: "M/D/YYYY",
			expected:    "6/28/2025",
		},
		{
			name:        "MM/DD/YYYY format",
			userPattern: "MM/DD/YYYY",
			expected:    "06/28/2025",
		},
		{
			name:        "YYYYMMDD format (log filename)",
			userPattern: "YYYYMMDD",
			expected:    "20250628",
		},
		{
			name:        "YYYY.MM.DD format",
			userPattern: "YYYY.MM.DD",
			expected:    "2025.06.28",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDateWithPattern(testTime, tt.userPattern)
			if result != tt.expected {
				t.Errorf("FormatDateWithPattern(%v, %q) = %q, want %q",
					testTime, tt.userPattern, result, tt.expected)
			}
		})
	}
}

func TestParseFlexibleDate(t *testing.T) {
	tests := []struct {
		name        string
		dateStr     string
		expectError bool
		expected    time.Time
	}{
		{
			name:        "M/D/YYYY format",
			dateStr:     "6/28/2025",
			expectError: false,
			expected:    time.Date(2025, 6, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "MM/DD/YYYY format",
			dateStr:     "06/28/2025",
			expectError: false,
			expected:    time.Date(2025, 6, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "YYYY-MM-DD format",
			dateStr:     "2025-06-28",
			expectError: false,
			expected:    time.Date(2025, 6, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "YYYYMMDD format",
			dateStr:     "20250628",
			expectError: false,
			expected:    time.Date(2025, 6, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:        "Invalid format",
			dateStr:     "invalid-date",
			expectError: true,
		},
		{
			name:        "Empty string",
			dateStr:     "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseFlexibleDate(tt.dateStr)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseFlexibleDate(%q) expected error, got nil", tt.dateStr)
				}
				return
			}
			
			if err != nil {
				t.Errorf("ParseFlexibleDate(%q) unexpected error: %v", tt.dateStr, err)
				return
			}
			
			if !result.Equal(tt.expected) {
				t.Errorf("ParseFlexibleDate(%q) = %v, want %v", tt.dateStr, result, tt.expected)
			}
		})
	}
}